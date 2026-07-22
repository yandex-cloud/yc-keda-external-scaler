package config

import (
	"strings"
	"testing"
	"time"
)

func validConfig() *Config {
	return &Config{
		IAMEndpoint:          "https://iam.example.test",
		MonitoringEndpoint:   "https://monitoring.example.test",
		AuthMethod:           "authorizedKey",
		KeyPath:              "/tmp/key.json",
		WLIFServiceAccountID: "service-account-id",
		WLIFTokenExchangeURL: "https://auth.example.test/oauth/token",
		WLIFSubjectTokenFile: "/tmp/subject-token",
		APITimeout:           time.Second,
	}
}

func TestValidateAuthenticationMethod(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*Config)
		wantErr   string
	}{
		{name: "authorized key"},
		{name: "authorized key requires IAM endpoint", configure: func(cfg *Config) { cfg.IAMEndpoint = "" }, wantErr: "IAM endpoint"},
		{name: "authorized key requires key path", configure: func(cfg *Config) { cfg.KeyPath = "" }, wantErr: "key path"},
		{name: "WLIF", configure: func(cfg *Config) { cfg.AuthMethod = "workloadIdentityFederation" }},
		{name: "WLIF does not require key settings", configure: func(cfg *Config) {
			cfg.AuthMethod = "workloadIdentityFederation"
			cfg.IAMEndpoint = ""
			cfg.KeyPath = ""
		}},
		{name: "WLIF requires service account", configure: func(cfg *Config) {
			cfg.AuthMethod = "workloadIdentityFederation"
			cfg.WLIFServiceAccountID = ""
		}, wantErr: "service account ID"},
		{name: "WLIF requires exchange URL", configure: func(cfg *Config) {
			cfg.AuthMethod = "workloadIdentityFederation"
			cfg.WLIFTokenExchangeURL = ""
		}, wantErr: "token exchange URL"},
		{name: "WLIF requires token file", configure: func(cfg *Config) {
			cfg.AuthMethod = "workloadIdentityFederation"
			cfg.WLIFSubjectTokenFile = ""
		}, wantErr: "subject token file"},
		{name: "unknown method", configure: func(cfg *Config) { cfg.AuthMethod = "unknown" }, wantErr: "unsupported authentication method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			if tt.configure != nil {
				tt.configure(cfg)
			}
			err := cfg.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
