package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"keda-external-scaler-yc-monitoring/internal/config"
)

func TestAuthorizedKeyProviderRegression(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	key := ServiceAccountKey{
		ID:               "key-id",
		ServiceAccountID: "service-account-id",
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyBytes,
		})),
	}
	keyBytes, _ := json.Marshal(key)
	keyPath := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(keyPath, keyBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	var requests int
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != "https://iam.example.test/iam/v1/tokens" {
			t.Errorf("request URL = %q", request.URL)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"jwt":"`) {
			t.Errorf("request body does not contain jwt")
		}
		return response(http.StatusOK, `{"iamToken":"authorized-key-token","expiresAt":"2099-01-01T00:00:00Z"}`), nil
	})}

	cfg := &config.Config{IAMEndpoint: "https://iam.example.test", APITimeout: time.Second}
	provider, err := NewYandexAuth(keyPath, cfg)
	if err != nil {
		t.Fatalf("NewYandexAuth() error = %v", err)
	}
	provider.httpClient = client

	for i := 0; i < 2; i++ {
		token, err := provider.GetToken(context.Background())
		if err != nil {
			t.Fatalf("GetToken() error = %v", err)
		}
		if token != "authorized-key-token" {
			t.Fatalf("GetToken() = %q", token)
		}
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}
