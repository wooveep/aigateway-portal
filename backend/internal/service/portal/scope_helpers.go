package portal

import (
	"context"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
)

func (s *Service) resolveScopeUser(ctx context.Context, consumerName string) (model.AuthUser, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	if normalizedConsumer == "" {
		return model.AuthUser{}, apperr.New(401, "unauthorized")
	}

	row, err := s.getUserByName(ctx, normalizedConsumer)
	if err != nil {
		return model.AuthUser{}, err
	}
	if row == nil {
		return model.AuthUser{}, apperr.New(404, "account not found")
	}
	orgContext, err := s.loadUserOrgContext(ctx, normalizedConsumer)
	if err != nil {
		return model.AuthUser{}, err
	}
	user := model.AuthUser{
		ConsumerName:       row.ConsumerName,
		DisplayName:        row.DisplayName,
		Email:              row.Email,
		DepartmentID:       orgContext.DepartmentID,
		DepartmentName:     orgContext.DepartmentName,
		DepartmentPath:     orgContext.DepartmentPath,
		ParentConsumerName: orgContext.ParentConsumerName,
		AdminConsumerName:  orgContext.AdminConsumerName,
		IsDepartmentAdmin:  orgContext.IsDepartmentAdmin,
		UserLevel:          normalizeUserLevel(row.UserLevel),
		Status:             row.Status,
	}
	if !strings.EqualFold(user.Status, consts.UserStatusActive) {
		return model.AuthUser{}, apperr.New(403, "account disabled")
	}
	return user, nil
}

func (s *Service) ResolveScopeUser(ctx context.Context, consumerName string) (model.AuthUser, error) {
	return s.resolveScopeUser(ctx, consumerName)
}

func (s *Service) applyModelRequestURL(item model.ModelInfo) model.ModelInfo {
	item.RequestURL = s.buildGatewayURL(item.Endpoint, "/v1/chat/completions", false)
	return item
}

func (s *Service) buildGatewayURL(endpoint string, fallbackPath string, internal bool) string {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if normalizedEndpoint == "" || normalizedEndpoint == "-" {
		normalizedEndpoint = strings.TrimSpace(fallbackPath)
	}
	if strings.HasPrefix(normalizedEndpoint, "http://") || strings.HasPrefix(normalizedEndpoint, "https://") {
		return normalizedEndpoint
	}
	if !strings.HasPrefix(normalizedEndpoint, "/") {
		normalizedEndpoint = "/" + normalizedEndpoint
	}

	baseURL := strings.TrimSpace(s.cfg.GatewayPublicBaseURL)
	if internal {
		baseURL = strings.TrimSpace(s.cfg.GatewayInternalBaseURL)
		if baseURL == "" {
			baseURL = strings.TrimSpace(s.cfg.GatewayPublicBaseURL)
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return normalizedEndpoint
	}
	return baseURL + normalizedEndpoint
}

func buildPortalMcpHTTPPath(mcpServerName string) string {
	return "/mcp-servers/" + url.PathEscape(strings.TrimSpace(mcpServerName))
}

func buildPortalMcpSSEPath(mcpServerName string) string {
	return buildPortalMcpHTTPPath(mcpServerName) + "/sse"
}

func parseJSONList(raw string) []string {
	return parseStringList(raw)
}

func trimPreview(text string, limit int) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r", ""), "\n", " "))
	if limit <= 0 || len([]rune(normalized)) <= limit {
		return normalized
	}
	runes := []rune(normalized)
	return strings.TrimSpace(string(runes[:limit]))
}

func requireNonBlankValue(value string, message string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", apperr.New(400, message)
	}
	return normalized, nil
}

func wrapInternalError(message string, err error) error {
	if err == nil {
		return nil
	}
	return gerror.Wrap(err, message)
}
