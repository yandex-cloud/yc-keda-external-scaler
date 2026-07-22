package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"keda-external-scaler-yc-monitoring/internal/config"
)

type ServiceAccountKey struct {
	ID               string `json:"id"`
	ServiceAccountID string `json:"service_account_id"`
	PrivateKey       string `json:"private_key"`
}

type YandexAuth struct {
	saKey      ServiceAccountKey
	tokenCache *tokenCacheEntry
	mutex      sync.RWMutex
	config     *config.Config
	httpClient *http.Client
}

type tokenCacheEntry struct {
	token     string
	expiresAt time.Time
}

type IAMTokenResponse struct {
	IAMToken  string    `json:"iamToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewYandexAuth(keyPath string, cfg *config.Config) (*YandexAuth, error) {
	key, err := loadServiceAccountKey(keyPath)
	if err != nil {
		return nil, err
	}
	return &YandexAuth{
		saKey:      key,
		config:     cfg,
		httpClient: &http.Client{Timeout: cfg.APITimeout},
	}, nil
}

func (y *YandexAuth) GetToken(ctx context.Context) (string, error) {
	y.mutex.RLock()
	if y.tokenCache != nil && time.Now().Before(y.tokenCache.expiresAt) {
		token := y.tokenCache.token
		y.mutex.RUnlock()
		return token, nil
	}
	y.mutex.RUnlock()

	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.tokenCache != nil && time.Now().Before(y.tokenCache.expiresAt) {
		return y.tokenCache.token, nil
	}

	token, expiresAt, err := y.createNewToken(ctx)
	if err != nil {
		return "", err
	}

	y.tokenCache = &tokenCacheEntry{
		token:     token,
		expiresAt: expiresAt,
	}

	return token, nil
}

func (y *YandexAuth) createNewToken(ctx context.Context) (string, time.Time, error) {
	jwtToken, err := y.createJWT()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create JWT: %v", err)
	}

	payload := map[string]string{"jwt": jwtToken}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, y.config.GetIAMTokenURL(), strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create IAM token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get IAM token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("IAM API error: %d, %s", resp.StatusCode, string(body))
	}

	var tokenResp IAMTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode IAM response: %v", err)
	}

	cacheExpiry := time.Now().Add(6 * time.Hour)
	if tokenResp.ExpiresAt.Before(cacheExpiry) {
		cacheExpiry = tokenResp.ExpiresAt.Add(-5 * time.Minute)
	}

	return tokenResp.IAMToken, cacheExpiry, nil
}

func (y *YandexAuth) createJWT() (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"aud": y.config.GetIAMTokenURL(),
		"iss": y.saKey.ServiceAccountID,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}

	block, _ := pem.Decode([]byte(y.saKey.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block containing the key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not an RSA private key")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodPS256, claims)
	token.Header["kid"] = y.saKey.ID

	return token.SignedString(rsaKey)
}

func (y *YandexAuth) CreateIAMToken() (string, error) {
	return y.GetToken(context.Background())
}

func loadServiceAccountKey(keyPath string) (ServiceAccountKey, error) {
	var key ServiceAccountKey

	data, err := os.ReadFile(keyPath)
	if err != nil {
		return key, fmt.Errorf("failed to read key file: %v", err)
	}

	if err := json.Unmarshal(data, &key); err != nil {
		return key, fmt.Errorf("failed to parse key file: %v", err)
	}

	return key, nil
}
