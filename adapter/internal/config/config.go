// Package config reads the adapter's configuration from the environment. Every knob has a
// default that is safe (fail closed) rather than convenient.
package config

import (
	"os"
	"strconv"
	"time"
)

// Version is set at build time with -ldflags "-X .../config.Version=<git sha>".
var Version = "dev"

type Config struct {
	GRPCAddr string
	HTTPAddr string
	// PublicURL is the gateway's public base URL, used to build approval links.
	PublicURL string

	OPAURL        string
	GuardrailsURL string
	ValkeyAddr    string // empty means in-memory stores (tests, single-process demos)

	ClickHouseURL      string // empty means audit to stdout
	ClickHouseUser     string
	ClickHousePassword string
	AuditRequired      bool
	ContentCapture     bool

	RouteModelExternal string
	RouteModelInhouse  string

	OIDCIssuer       string
	OIDCInternalURL  string
	OIDCClientID     string
	OIDCClientSecret string
	CookieSecret     string

	ApprovalTTL       time.Duration
	TaintTTL          time.Duration
	RouteCacheTTL     time.Duration
	GuardrailsTimeout time.Duration
	OPATimeout        time.Duration
	AuditTimeout      time.Duration
}

func FromEnv() Config {
	return Config{
		GRPCAddr:           env("EACP_GRPC_ADDR", ":9001"),
		HTTPAddr:           env("EACP_HTTP_ADDR", ":8080"),
		PublicURL:          env("EACP_PUBLIC_URL", "http://localhost:8080"),
		OPAURL:             env("EACP_OPA_URL", "http://127.0.0.1:8181"),
		GuardrailsURL:      env("EACP_GUARDRAILS_URL", "http://127.0.0.1:8000"),
		ValkeyAddr:         env("EACP_VALKEY_ADDR", ""),
		ClickHouseURL:      env("EACP_CLICKHOUSE_URL", ""),
		ClickHouseUser:     env("EACP_CLICKHOUSE_USER", ""),
		ClickHousePassword: env("EACP_CLICKHOUSE_PASSWORD", ""),
		AuditRequired:      envBool("EACP_AUDIT_REQUIRED", true),
		ContentCapture:     envBool("EACP_CONTENT_CAPTURE", false),
		RouteModelExternal: env("EACP_ROUTE_MODEL_EXTERNAL", "eacp-external"),
		RouteModelInhouse:  env("EACP_ROUTE_MODEL_INHOUSE", "eacp-inhouse"),
		OIDCIssuer:         env("EACP_OIDC_ISSUER", ""),
		OIDCInternalURL:    env("EACP_OIDC_INTERNAL_URL", ""),
		OIDCClientID:       env("EACP_OIDC_CLIENT_ID", "eacp-adapter"),
		OIDCClientSecret:   env("EACP_OIDC_CLIENT_SECRET", ""),
		CookieSecret:       env("EACP_COOKIE_SECRET", ""),
		ApprovalTTL:        envDuration("EACP_APPROVAL_TTL", 15*time.Minute),
		TaintTTL:           envDuration("EACP_TAINT_TTL", 30*time.Minute),
		RouteCacheTTL:      envDuration("EACP_ROUTE_CACHE_TTL", 5*time.Minute),
		GuardrailsTimeout:  envDuration("EACP_GUARDRAILS_TIMEOUT", 2*time.Second),
		OPATimeout:         envDuration("EACP_OPA_TIMEOUT", 500*time.Millisecond),
		AuditTimeout:       envDuration("EACP_AUDIT_TIMEOUT", 1*time.Second),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
