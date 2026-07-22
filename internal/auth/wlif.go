package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"keda-external-scaler-yc-monitoring/internal/config"
)

const (
	tokenExchangeGrantType       = "urn:ietf:params:oauth:grant-type:token-exchange"
	tokenExchangeAccessTokenType = "urn:ietf:params:oauth:token-type:access_token"
	tokenExchangeIDTokenType     = "urn:ietf:params:oauth:token-type:id_token"
)

type workloadIdentityTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type workloadIdentityErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type WorkloadIdentityProvider struct {
	serviceAccountID string
	exchangeURL      string
	subjectTokenFile string
	httpClient       *http.Client
	now              func() time.Time

	mutex      sync.Mutex
	tokenCache *tokenCacheEntry
}

func NewWorkloadIdentityProvider(cfg *config.Config) *WorkloadIdentityProvider {
	return &WorkloadIdentityProvider{
		serviceAccountID: cfg.WLIFServiceAccountID,
		exchangeURL:      cfg.WLIFTokenExchangeURL,
		subjectTokenFile: cfg.WLIFSubjectTokenFile,
		httpClient:       &http.Client{Timeout: cfg.APITimeout},
		now:              time.Now,
	}
}

func (p *WorkloadIdentityProvider) GetToken(ctx context.Context) (string, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := p.now()
	if p.tokenCache != nil && now.Before(p.tokenCache.expiresAt) {
		return p.tokenCache.token, nil
	}

	subjectToken, err := os.ReadFile(p.subjectTokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read WLIF subject token: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrantType)
	form.Set("requested_token_type", tokenExchangeAccessTokenType)
	form.Set("audience", p.serviceAccountID)
	form.Set("subject_token", strings.TrimSpace(string(subjectToken)))
	form.Set("subject_token_type", tokenExchangeIDTokenType)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.exchangeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create WLIF token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange WLIF subject token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", decodeWorkloadIdentityError(resp)
	}

	var tokenResponse workloadIdentityTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("failed to decode WLIF token exchange response: %w", err)
	}
	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("WLIF token exchange response does not contain access_token")
	}

	ttl := time.Duration(tokenResponse.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	refreshMargin := 5 * time.Minute
	if ttl <= refreshMargin {
		refreshMargin = ttl / 10
	}

	p.tokenCache = &tokenCacheEntry{
		token:     tokenResponse.AccessToken,
		expiresAt: now.Add(ttl - refreshMargin),
	}
	return tokenResponse.AccessToken, nil
}

func decodeWorkloadIdentityError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("WLIF token exchange failed with status %s", resp.Status)
	}

	var errorResponse workloadIdentityErrorResponse
	if json.Unmarshal(body, &errorResponse) == nil {
		switch {
		case errorResponse.Error != "" && errorResponse.ErrorDescription != "":
			return fmt.Errorf("WLIF token exchange failed with status %s: %s (%s)", resp.Status, errorResponse.Error, errorResponse.ErrorDescription)
		case errorResponse.Error != "":
			return fmt.Errorf("WLIF token exchange failed with status %s: %s", resp.Status, errorResponse.Error)
		}
	}

	return fmt.Errorf("WLIF token exchange failed with status %s", resp.Status)
}
