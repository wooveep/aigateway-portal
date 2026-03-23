package portal

import "higress-portal-backend/internal/config"

func (s *Service) Config() config.Config {
	return s.cfg
}
