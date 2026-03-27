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
	MySQLDSN                     string
	MySQLHost                    string
	MySQLPort                    string
	MySQLUser                    string
	MySQLPassword                string
	MySQLDatabase                string
	MySQLParams                  string
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
	WebRoot                      string
}

func Load() Config {
	cfg := Config{
		ListenAddr:                   getEnv("PORTAL_LISTEN_ADDR", ":8080"),
		MySQLDSN:                     getEnv("PORTAL_MYSQL_DSN", ""),
		MySQLHost:                    getEnv("PORTAL_MYSQL_HOST", "127.0.0.1"),
		MySQLPort:                    getEnv("PORTAL_MYSQL_PORT", "3306"),
		MySQLUser:                    getEnv("PORTAL_MYSQL_USER", "root"),
		MySQLPassword:                getEnv("PORTAL_MYSQL_PASSWORD", "root"),
		MySQLDatabase:                getEnv("PORTAL_MYSQL_DATABASE", "aigateway_portal"),
		MySQLParams:                  getEnv("PORTAL_MYSQL_PARAMS", "parseTime=true&charset=utf8mb4&loc=Local"),
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
		WebRoot:                      getEnv("PORTAL_WEB_ROOT", "/app/web"),
	}
	if cfg.MySQLDSN == "" && !hasPortalMySQLConnEnv() {
		applySharedPortalDB(&cfg)
	}
	if cfg.MySQLDSN == "" {
		cfg.MySQLDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
			cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase, cfg.MySQLParams,
		)
	}
	if strings.TrimSpace(cfg.K8sNamespace) == "" {
		cfg.K8sNamespace = "aigateway-system"
	}
	return cfg
}

func hasPortalMySQLConnEnv() bool {
	keys := []string{
		"PORTAL_MYSQL_HOST",
		"PORTAL_MYSQL_PORT",
		"PORTAL_MYSQL_USER",
		"PORTAL_MYSQL_PASSWORD",
		"PORTAL_MYSQL_DATABASE",
		"PORTAL_MYSQL_PARAMS",
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
	host, port, database, params, err := parseMySQLJDBCURL(rawURL)
	if err != nil {
		return
	}
	cfg.MySQLHost = host
	cfg.MySQLPort = port
	cfg.MySQLDatabase = database
	cfg.MySQLParams = params

	if username := firstNonEmptyEnv("PORTAL_CORE_DB_USERNAME", "HIGRESS_PORTAL_DB_USERNAME"); username != "" {
		cfg.MySQLUser = username
	}
	if password := firstNonEmptyEnv("PORTAL_CORE_DB_PASSWORD", "HIGRESS_PORTAL_DB_PASSWORD"); password != "" {
		cfg.MySQLPassword = password
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

func parseMySQLJDBCURL(raw string) (host string, port string, database string, params string, err error) {
	const prefix = "jdbc:mysql://"
	if !strings.HasPrefix(strings.ToLower(raw), prefix) {
		return "", "", "", "", fmt.Errorf("unsupported jdbc mysql url")
	}
	parsed, err := url.Parse("mysql://" + raw[len(prefix):])
	if err != nil {
		return "", "", "", "", err
	}
	host = parsed.Hostname()
	if host == "" {
		return "", "", "", "", fmt.Errorf("empty mysql host")
	}
	port = parsed.Port()
	if port == "" {
		port = "3306"
	}
	database = strings.TrimPrefix(parsed.Path, "/")
	if database == "" {
		return "", "", "", "", fmt.Errorf("empty mysql database")
	}
	params = ensureGoMySQLParams(parsed.RawQuery)
	return host, port, database, params, nil
}

func ensureGoMySQLParams(raw string) string {
	const defaultParams = "parseTime=true&charset=utf8mb4&loc=Local"
	if strings.TrimSpace(raw) == "" {
		return defaultParams
	}
	query, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	if query.Get("parseTime") == "" {
		query.Set("parseTime", "true")
	}
	if query.Get("charset") == "" {
		query.Set("charset", "utf8mb4")
	}
	if query.Get("loc") == "" {
		query.Set("loc", "Local")
	}
	return query.Encode()
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
