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

func (s *Service) applyModelRequestURL(ctx context.Context, item model.ModelInfo) model.ModelInfo {
	resolver := s.newGatewayAddressResolver(ctx)
	item.RequestURL = resolver.resolveURL(item.Endpoint, "/v1/chat/completions", false)
	item.InternalRouteURL = ""
	if strings.TrimSpace(item.InternalEndpoint) != "" {
		item.InternalRouteURL = resolver.resolveURL(item.InternalEndpoint, "/v1/chat/completions", true)
	}
	return item
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
