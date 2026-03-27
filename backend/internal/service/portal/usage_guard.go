package portal

import (
	"strings"

	"higress-portal-backend/internal/apperr"
)

func (s *Service) ensureUsageSourceConfigured() error {
	if strings.TrimSpace(s.cfg.CorePrometheusURL) == "" {
		return apperr.New(503, "usage statistics unavailable: PORTAL_CORE_PROMETHEUS_URL is not configured")
	}
	return nil
}
