package auth

import (
	"context"
	"fmt"

	"keda-external-scaler-yc-monitoring/internal/config"
)

type TokenProvider interface {
	GetToken(context.Context) (string, error)
}

func NewTokenProvider(keyPath string, cfg *config.Config) (TokenProvider, error) {
	switch cfg.AuthMethod {
	case "authorizedKey":
		return NewYandexAuth(keyPath, cfg)
	case "workloadIdentityFederation":
		return NewWorkloadIdentityProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported authentication method %q", cfg.AuthMethod)
	}
}
