package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	IAMEndpoint        string
	MonitoringEndpoint string
	AuthMethod         string

	WLIFServiceAccountID string
	WLIFTokenExchangeURL string
	WLIFSubjectTokenFile string

	GRPCPort   string
	HTTPPort   string
	HealthPath string

	KeyPath string

	APITimeout time.Duration
}

func LoadConfig() *Config {
	return &Config{
		IAMEndpoint:        getEnv("IAM_ENDPOINT", "https://iam.api.cloud.yandex.net"),
		MonitoringEndpoint: getEnv("MONITORING_ENDPOINT", "https://monitoring.api.cloud.yandex.net"),
		AuthMethod:         getEnv("AUTH_METHOD", "authorizedKey"),

		WLIFServiceAccountID: getEnv("WLIF_SERVICE_ACCOUNT_ID", ""),
		WLIFTokenExchangeURL: getEnv("WLIF_TOKEN_EXCHANGE_URL", "https://auth.yandex.cloud/oauth/token"),
		WLIFSubjectTokenFile: getEnv("WLIF_SUBJECT_TOKEN_FILE", "/var/run/secrets/tokens/yc-wlif-token"),

		GRPCPort:   getEnv("GRPC_PORT", "8080"),
		HTTPPort:   getEnv("HTTP_PORT", "8081"),
		HealthPath: getEnv("HEALTH_PATH", "/health"),

		KeyPath: getEnv("KEY_PATH", "/app/key.json"),

		APITimeout: parseDurationWithDefault("API_TIMEOUT", 30*time.Second),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func (c *Config) Validate() error {
	if c.MonitoringEndpoint == "" {
		return fmt.Errorf("monitoring endpoint cannot be empty")
	}

	switch c.AuthMethod {
	case "authorizedKey":
		if c.IAMEndpoint == "" {
			return fmt.Errorf("IAM endpoint cannot be empty")
		}
		if c.KeyPath == "" {
			return fmt.Errorf("key path cannot be empty")
		}
	case "workloadIdentityFederation":
		if c.WLIFServiceAccountID == "" {
			return fmt.Errorf("WLIF service account ID cannot be empty")
		}
		if c.WLIFTokenExchangeURL == "" {
			return fmt.Errorf("WLIF token exchange URL cannot be empty")
		}
		if c.WLIFSubjectTokenFile == "" {
			return fmt.Errorf("WLIF subject token file cannot be empty")
		}
	default:
		return fmt.Errorf("unsupported authentication method %q", c.AuthMethod)
	}
	return nil
}

func (c *Config) GetIAMTokenURL() string {
	return c.IAMEndpoint + "/iam/v1/tokens"
}

func (c *Config) GetMonitoringURL(folderID string) string {
	return fmt.Sprintf("%s/monitoring/v2/data/read?folderId=%s",
		c.MonitoringEndpoint, folderID)
}
