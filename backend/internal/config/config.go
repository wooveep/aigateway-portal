package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr                   string
	DBDriver                     string
	DBAutoMigrate                bool
	DBDSN                        string
	DBHost                       string
	DBPort                       string
	DBUser                       string
	DBPassword                   string
	DBName                       string
	DBParams                     string
	K8sNamespace                 string
	KubeConfigPath               string
	SessionCookieName            string
	SessionSecret                string
	SessionTTL                   time.Duration
	SessionSecureCookie          bool
	InviteCode                   string
	InviteExpireDays             int
	CorePrometheusURL            string
	UsageSyncInterval            time.Duration
	UsageSyncEnabled             bool
	KeyAuthSyncInterval          time.Duration
	KeyAuthSyncEnabled           bool
	BillingSyncInterval          time.Duration
	BillingSyncEnabled           bool
	BillingConsumerBlock         time.Duration
	BillingConsumerBatchSize     int
	RechargeFallbackAvgCostPer1K float64
	ShutdownGracePeriod          time.Duration
	PortalDisplayName            string
	GatewayPublicBaseURL         string
	GatewayInternalBaseURL       string
	GatewayPublicHostFallback    string
	GatewayServiceName           string
	WebRoot                      string
}

func Load() Config {
	cfg := Config{
		ListenAddr:                   getEnv("PORTAL_LISTEN_ADDR", ":8080"),
		DBDriver:                     normalizeDBDriver(getEnv("PORTAL_DB_DRIVER", "postgres")),
		DBAutoMigrate:                getEnvBool("PORTAL_DB_AUTO_MIGRATE", true),
		DBDSN:                        getEnv("PORTAL_DB_DSN", ""),
		DBHost:                       getEnv("PORTAL_DB_HOST", "127.0.0.1"),
		DBPort:                       getEnv("PORTAL_DB_PORT", "5432"),
		DBUser:                       getEnv("PORTAL_DB_USER", "postgres"),
		DBPassword:                   getEnv("PORTAL_DB_PASSWORD", "postgres"),
		DBName:                       getEnv("PORTAL_DB_NAME", "aigateway_portal"),
		DBParams:                     getEnv("PORTAL_DB_PARAMS", "sslmode=disable"),
		K8sNamespace:                 firstNonEmptyEnv("PORTAL_K8S_NAMESPACE", "POD_NAMESPACE"),
		KubeConfigPath:               firstNonEmptyEnv("PORTAL_K8S_KUBECONFIG", "KUBECONFIG"),
		SessionCookieName:            getEnv("PORTAL_SESSION_COOKIE_NAME", "aigateway_portal_session"),
		SessionSecret:                getEnv("PORTAL_SESSION_SECRET", "dev-session-secret"),
		SessionTTL:                   time.Duration(getEnvInt("PORTAL_SESSION_TTL_HOURS", 72)) * time.Hour,
		SessionSecureCookie:          getEnvBool("PORTAL_SESSION_SECURE", false),
		InviteCode:                   getEnv("PORTAL_BOOTSTRAP_INVITE_CODE", ""),
		InviteExpireDays:             getEnvInt("PORTAL_BOOTSTRAP_INVITE_EXPIRE_DAYS", 365),
		CorePrometheusURL:            getEnv("PORTAL_CORE_PROMETHEUS_URL", ""),
		UsageSyncInterval:            time.Duration(getEnvInt("PORTAL_USAGE_SYNC_SECONDS", 300)) * time.Second,
		UsageSyncEnabled:             getEnvBool("PORTAL_USAGE_SYNC_ENABLED", true),
		KeyAuthSyncInterval:          time.Duration(getEnvInt("PORTAL_KEYAUTH_SYNC_SECONDS", 2)) * time.Second,
		KeyAuthSyncEnabled:           getEnvBool("PORTAL_KEYAUTH_SYNC_ENABLED", true),
		BillingSyncInterval:          time.Duration(getEnvInt("PORTAL_BILLING_SYNC_SECONDS", 15)) * time.Second,
		BillingSyncEnabled:           getEnvBool("PORTAL_BILLING_SYNC_ENABLED", true),
		BillingConsumerBlock:         time.Duration(getEnvInt("PORTAL_BILLING_CONSUMER_BLOCK_SECONDS", 5)) * time.Second,
		BillingConsumerBatchSize:     getEnvInt("PORTAL_BILLING_CONSUMER_BATCH_SIZE", 20),
		RechargeFallbackAvgCostPer1K: getEnvFloat("PORTAL_RECHARGE_FALLBACK_AVG_COST_PER_1K", 0.02),
		ShutdownGracePeriod:          time.Duration(getEnvInt("PORTAL_SHUTDOWN_GRACE_SECONDS", 10)) * time.Second,
		PortalDisplayName:            getEnv("PORTAL_DISPLAY_NAME", "AIGateway 用户门户"),
		GatewayPublicBaseURL:         strings.TrimRight(getEnv("PORTAL_GATEWAY_PUBLIC_BASE_URL", ""), "/"),
		GatewayInternalBaseURL:       strings.TrimRight(getEnv("PORTAL_GATEWAY_INTERNAL_BASE_URL", ""), "/"),
		GatewayPublicHostFallback:    strings.TrimSpace(getEnv("PORTAL_GATEWAY_PUBLIC_HOST_FALLBACK", "")),
		GatewayServiceName:           strings.TrimSpace(getEnv("PORTAL_GATEWAY_SERVICE_NAME", "aigateway-gateway")),
		WebRoot:                      getEnv("PORTAL_WEB_ROOT", "/app/web"),
	}
	if cfg.DBDSN == "" && !hasPortalGenericConnEnv() {
		applySharedPortalDB(&cfg)
	}
	if cfg.DBDSN == "" {
		cfg.DBDSN = buildPostgresDSN(
			cfg.DBHost,
			cfg.DBPort,
			cfg.DBUser,
			cfg.DBPassword,
			cfg.DBName,
			cfg.DBParams,
		)
	}
	if strings.TrimSpace(cfg.K8sNamespace) == "" {
		cfg.K8sNamespace = "aigateway-system"
	}
	if cfg.GatewayServiceName == "" {
		cfg.GatewayServiceName = "aigateway-gateway"
	}
	if cfg.GatewayInternalBaseURL == "" {
		cfg.GatewayInternalBaseURL = cfg.GatewayPublicBaseURL
	}
	return cfg
}

func hasPortalGenericConnEnv() bool {
	keys := []string{
		"PORTAL_DB_HOST",
		"PORTAL_DB_PORT",
		"PORTAL_DB_USER",
		"PORTAL_DB_PASSWORD",
		"PORTAL_DB_NAME",
		"PORTAL_DB_PARAMS",
	}
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func applySharedPortalDB(cfg *Config) {
	rawURL := firstNonEmptyEnv("PORTAL_CORE_DB_URL", "HIGRESS_PORTAL_DB_URL")
	if rawURL == "" {
		return
	}
	host, port, database, params, err := parsePostgresJDBCURL(rawURL)
	if err != nil {
		return
	}

	cfg.DBDriver = "postgres"
	cfg.DBHost = host
	cfg.DBPort = port
	cfg.DBName = database
	cfg.DBParams = params

	if username := firstNonEmptyEnv("PORTAL_CORE_DB_USERNAME", "HIGRESS_PORTAL_DB_USERNAME"); username != "" {
		cfg.DBUser = username
	}
	if password := firstNonEmptyEnv("PORTAL_CORE_DB_PASSWORD", "HIGRESS_PORTAL_DB_PASSWORD"); password != "" {
		cfg.DBPassword = password
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func parsePostgresJDBCURL(raw string) (host string, port string, database string, params string, err error) {
	const prefix = "jdbc:postgresql://"
	if !strings.HasPrefix(strings.ToLower(raw), prefix) {
		return "", "", "", "", fmt.Errorf("unsupported jdbc postgresql url")
	}
	parsed, err := url.Parse("postgres://" + raw[len(prefix):])
	if err != nil {
		return "", "", "", "", err
	}
	host = parsed.Hostname()
	if host == "" {
		return "", "", "", "", fmt.Errorf("empty postgresql host")
	}
	port = parsed.Port()
	if port == "" {
		port = "5432"
	}
	database = strings.TrimPrefix(parsed.Path, "/")
	if database == "" {
		return "", "", "", "", fmt.Errorf("empty postgresql database")
	}
	params = ensurePostgresParams(parsed.RawQuery)
	return host, port, database, params, nil
}

func buildPostgresDSN(host, port, user, password, database, params string) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s %s",
		host,
		port,
		user,
		password,
		database,
		ensurePostgresParams(params),
	)
}

func ensurePostgresParams(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "sslmode=disable"
	}
	query, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	if query.Get("sslmode") == "" {
		query.Set("sslmode", "disable")
	}
	return strings.ReplaceAll(query.Encode(), "&", " ")
}

func normalizeDBDriver(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "postgres", "postgresql", "pgx", "pgsql":
		return "postgres"
	default:
		return "postgres"
	}
}

func getEnv(key string, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return i
}

func getEnvBool(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getEnvFloat(key string, defaultValue float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}
