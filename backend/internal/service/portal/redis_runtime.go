package portal

import (
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	clientK8s "higress-portal-backend/internal/client/k8s"
)

func newRedisClient(cfg clientK8s.AIQuotaRedisConfig) *redis.Client {
	timeout := time.Duration(cfg.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.ServiceName, cfg.ServicePort),
		Username:     strings.TrimSpace(cfg.Username),
		Password:     cfg.Password,
		DB:           cfg.Database,
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	})
}
