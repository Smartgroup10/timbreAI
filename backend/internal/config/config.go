package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	JWTTTL          time.Duration
	MigrationsDir   string
	AllowedOrigins  []string
	BootstrapAdmin  Credentials
	BootstrapTenant Credentials

	ARI          ARIConfig
	SIP          SIPConfig
	VoiceAgent   VoiceAgentConfig
	Storage      StorageConfig
}

type StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	PublicURL string
}

type VoiceAgentConfig struct {
	URL    string
	Secret string
}

type Credentials struct {
	Email    string
	Name     string
	Password string
	TenantID string
	Role     string
}

type ARIConfig struct {
	Enabled  bool
	URL      string
	User     string
	Password string
	App      string
}

type SIPConfig struct {
	TestExtension    string
	CallerID         string
	OriginateTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:          env("PORT", "8080"),
		DatabaseURL:   env("DATABASE_URL", ""),
		JWTSecret:     env("JWT_SECRET", ""),
		JWTTTL:        envDuration("JWT_TTL", 12*time.Hour),
		MigrationsDir: env("MIGRATIONS_DIR", "./migrations"),
		AllowedOrigins: splitCSV(env("ALLOWED_ORIGINS", "http://localhost:3000")),
		BootstrapAdmin: Credentials{
			Email:    env("BOOTSTRAP_ADMIN_EMAIL", "admin@timbre.ai"),
			Name:     env("BOOTSTRAP_ADMIN_NAME", "Platform Admin"),
			Password: env("BOOTSTRAP_ADMIN_PASSWORD", ""),
			Role:     "platform_admin",
		},
		BootstrapTenant: Credentials{
			Email:    env("BOOTSTRAP_TENANT_EMAIL", "owner@atrium.local"),
			Name:     env("BOOTSTRAP_TENANT_NAME", "Tenant Owner"),
			Password: env("BOOTSTRAP_TENANT_PASSWORD", ""),
			TenantID: env("BOOTSTRAP_TENANT_ID", "atrium"),
			Role:     "tenant_admin",
		},
		ARI: ARIConfig{
			Enabled:  envBool("ASTERISK_ARI_ENABLED", false),
			URL:      env("ASTERISK_ARI_URL", "http://asterisk:8088/ari"),
			User:     env("ASTERISK_ARI_USER", "timbre"),
			Password: env("ASTERISK_ARI_PASSWORD", ""),
			App:      env("ASTERISK_ARI_APP", "timbre-bot"),
		},
		SIP: SIPConfig{
			TestExtension:    env("SIP_TEST_EXTENSION", "PJSIP/6001"),
			CallerID:         env("SIP_CALLER_ID", "timbre.ai <1000>"),
			OriginateTimeout: envDuration("SIP_ORIGINATE_TIMEOUT", 30*time.Second),
		},
		VoiceAgent: VoiceAgentConfig{
			URL:    env("VOICE_AGENT_URL", ""),
			Secret: env("VOICE_AGENT_SHARED_SECRET", ""),
		},
		Storage: StorageConfig{
			Endpoint:  env("STORAGE_ENDPOINT", ""),
			AccessKey: env("STORAGE_ACCESS_KEY", ""),
			SecretKey: env("STORAGE_SECRET_KEY", ""),
			Bucket:    env("STORAGE_BUCKET", "timbre-recordings"),
			Region:    env("STORAGE_REGION", "us-east-1"),
			PublicURL: env("STORAGE_PUBLIC_URL", ""),
		},
	}

	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me" {
		return cfg, errors.New("JWT_SECRET must be set to a non-default value")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
