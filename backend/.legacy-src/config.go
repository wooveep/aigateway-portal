package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr          string
	MySQLDSN            string
	MySQLHost           string
	MySQLPort           string
	MySQLUser           string
	MySQLPassword       string
	MySQLDatabase       string
	MySQLParams         string
	SessionCookieName   string
	SessionSecret       string
	SessionTTL          time.Duration
	SessionSecureCookie bool
	InviteCode          string
	InviteExpireDays    int
	ConsoleBaseURL      string
	ConsoleUsername     string
	ConsolePassword     string
	UsageSyncInterval   time.Duration
	UsageSyncEnabled    bool
	ShutdownGracePeriod time.Duration
	PortalDisplayName   string
	WebRoot             string
}

func loadConfig() Config {
	cfg := Config{
		ListenAddr:          getEnv("PORTAL_LISTEN_ADDR", ":8080"),
		MySQLDSN:            getEnv("PORTAL_MYSQL_DSN", ""),
		MySQLHost:           getEnv("PORTAL_MYSQL_HOST", "127.0.0.1"),
		MySQLPort:           getEnv("PORTAL_MYSQL_PORT", "3306"),
		MySQLUser:           getEnv("PORTAL_MYSQL_USER", "root"),
		MySQLPassword:       getEnv("PORTAL_MYSQL_PASSWORD", "root"),
		MySQLDatabase:       getEnv("PORTAL_MYSQL_DATABASE", "aigateway_portal"),
		MySQLParams:         getEnv("PORTAL_MYSQL_PARAMS", "parseTime=true&charset=utf8mb4&loc=Local"),
		SessionCookieName:   getEnv("PORTAL_SESSION_COOKIE_NAME", "aigateway_portal_session"),
		SessionSecret:       getEnv("PORTAL_SESSION_SECRET", "dev-session-secret"),
		SessionTTL:          time.Duration(getEnvInt("PORTAL_SESSION_TTL_HOURS", 72)) * time.Hour,
		SessionSecureCookie: getEnvBool("PORTAL_SESSION_SECURE", false),
		InviteCode:          getEnv("PORTAL_BOOTSTRAP_INVITE_CODE", "WELCOME-2026"),
		InviteExpireDays:    getEnvInt("PORTAL_BOOTSTRAP_INVITE_EXPIRE_DAYS", 365),
		ConsoleBaseURL:      getEnv("HIGRESS_CONSOLE_URL", "http://aigateway-console:8080"),
		ConsoleUsername:     getEnv("HIGRESS_CONSOLE_USER", "admin"),
		ConsolePassword:     getEnv("HIGRESS_CONSOLE_PASSWORD", "admin"),
		UsageSyncInterval:   time.Duration(getEnvInt("PORTAL_USAGE_SYNC_SECONDS", 300)) * time.Second,
		UsageSyncEnabled:    getEnvBool("PORTAL_USAGE_SYNC_ENABLED", true),
		ShutdownGracePeriod: time.Duration(getEnvInt("PORTAL_SHUTDOWN_GRACE_SECONDS", 10)) * time.Second,
		PortalDisplayName:   getEnv("PORTAL_DISPLAY_NAME", "AIGateway 用户门户"),
		WebRoot:             getEnv("PORTAL_WEB_ROOT", "/app/web"),
	}
	if cfg.MySQLDSN == "" {
		cfg.MySQLDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
			cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase, cfg.MySQLParams)
	}
	return cfg
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
