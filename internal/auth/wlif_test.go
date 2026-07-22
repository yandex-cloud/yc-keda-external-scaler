package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"keda-external-scaler-yc-monitoring/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func writeSubjectToken(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subject-token")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatalf("write subject token: %v", err)
	}
	return path
}

func testWLIFConfig(tokenPath string) *config.Config {
	return &config.Config{
		WLIFServiceAccountID: "service-account-id",
		WLIFTokenExchangeURL: "https://auth.example.test/oauth/token",
		WLIFSubjectTokenFile: tokenPath,
		APITimeout:           time.Second,
	}
}

func TestWorkloadIdentityProviderExchangeAndCache(t *testing.T) {
	tokenPath := writeSubjectToken(t, "  subject-token-one\n")
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		if got := request.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}
		want := map[string]string{
			"grant_type":           tokenExchangeGrantType,
			"requested_token_type": tokenExchangeAccessTokenType,
			"audience":             "service-account-id",
			"subject_token":        "subject-token-one",
			"subject_token_type":   tokenExchangeIDTokenType,
		}
		for key, value := range want {
			if got := form.Get(key); got != value {
				t.Errorf("form[%q] = %q, want %q", key, got, value)
			}
		}
		return response(http.StatusOK, `{"access_token":"iam-token","expires_in":3600}`), nil
	})}

	now := time.Date(2026, time.July, 16, 10, 0, 0, 0, time.UTC)
	provider := NewWorkloadIdentityProvider(testWLIFConfig(tokenPath))
	provider.httpClient = client
	provider.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		token, err := provider.GetToken(context.Background())
		if err != nil {
			t.Fatalf("GetToken() error = %v", err)
		}
		if token != "iam-token" {
			t.Fatalf("GetToken() = %q", token)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("exchange requests = %d, want 1", got)
	}
}

func TestWorkloadIdentityProviderRefreshesAndRereadsSubjectToken(t *testing.T) {
	tokenPath := writeSubjectToken(t, "subject-token-one")
	var mutex sync.Mutex
	var subjects []string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		form, _ := url.ParseQuery(string(body))
		mutex.Lock()
		subjects = append(subjects, form.Get("subject_token"))
		mutex.Unlock()
		return response(http.StatusOK, `{"access_token":"iam-token","expires_in":600}`), nil
	})}

	now := time.Date(2026, time.July, 16, 10, 0, 0, 0, time.UTC)
	provider := NewWorkloadIdentityProvider(testWLIFConfig(tokenPath))
	provider.httpClient = client
	provider.now = func() time.Time { return now }

	if _, err := provider.GetToken(context.Background()); err != nil {
		t.Fatalf("first GetToken() error = %v", err)
	}
	if err := os.WriteFile(tokenPath, []byte("subject-token-two"), 0600); err != nil {
		t.Fatalf("rotate subject token: %v", err)
	}
	now = now.Add(9*time.Minute + time.Second)
	if _, err := provider.GetToken(context.Background()); err != nil {
		t.Fatalf("refreshed GetToken() error = %v", err)
	}

	mutex.Lock()
	defer mutex.Unlock()
	if strings.Join(subjects, ",") != "subject-token-one,subject-token-two" {
		t.Fatalf("subjects = %v", subjects)
	}
}

func TestWorkloadIdentityProviderConcurrentRefresh(t *testing.T) {
	tokenPath := writeSubjectToken(t, "subject-token")
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests.Add(1)
		time.Sleep(20 * time.Millisecond)
		return response(http.StatusOK, `{"access_token":"iam-token","expires_in":3600}`), nil
	})}
	provider := NewWorkloadIdentityProvider(testWLIFConfig(tokenPath))
	provider.httpClient = client

	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := provider.GetToken(context.Background()); err != nil {
				t.Errorf("GetToken() error = %v", err)
			}
		}()
	}
	group.Wait()
	if got := requests.Load(); got != 1 {
		t.Fatalf("exchange requests = %d, want 1", got)
	}
}

func TestWorkloadIdentityProviderErrors(t *testing.T) {
	tests := []struct {
		name        string
		missingFile bool
		transport   roundTripFunc
		want        string
	}{
		{name: "missing subject token", missingFile: true, want: "failed to read WLIF subject token"},
		{name: "OAuth error", want: "invalid_grant (subject rejected)", transport: func(_ *http.Request) (*http.Response, error) {
			return response(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"subject rejected"}`), nil
		}},
		{name: "malformed response", want: "failed to decode", transport: func(_ *http.Request) (*http.Response, error) {
			return response(http.StatusOK, "not-json"), nil
		}},
		{name: "empty token", want: "does not contain access_token", transport: func(_ *http.Request) (*http.Response, error) {
			return response(http.StatusOK, `{"expires_in":3600}`), nil
		}},
		{name: "timeout", want: "failed to exchange", transport: func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenPath := filepath.Join(t.TempDir(), "missing")
			if !tt.missingFile {
				if err := os.WriteFile(tokenPath, []byte("subject"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cfg := testWLIFConfig(tokenPath)
			provider := NewWorkloadIdentityProvider(cfg)
			if tt.transport != nil {
				provider.httpClient = &http.Client{Transport: tt.transport, Timeout: 10 * time.Millisecond}
			}
			_, err := provider.GetToken(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GetToken() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
