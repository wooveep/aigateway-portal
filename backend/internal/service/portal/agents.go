package portal

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/model"
)

type publishedAgentRecord struct {
	AgentID string
	Info    model.AgentInfo
}

func (s *Service) ListAgents(ctx context.Context, user model.AuthUser) ([]model.AgentInfo, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			agent_id,
			canonical_name,
			display_name,
			intro,
			description,
			icon_url,
			tags_json,
			mcp_server_name,
			tool_count,
			transport_types_json,
			resource_summary,
			prompt_summary,
			published_at,
			updated_at
		FROM portal_agent_catalog
		WHERE status = 'published'
		ORDER BY canonical_name ASC, display_name ASC`)
	if err != nil {
		return nil, apperr.New(503, "agent catalog unavailable", err.Error())
	}
	items := make([]publishedAgentRecord, 0, len(records))
	for _, record := range records {
		items = append(items, publishedAgentRecord{
			AgentID: strings.TrimSpace(record["agent_id"].String()),
			Info:    s.toPortalAgentInfo(record),
		})
	}
	visible, err := s.filterVisiblePublishedAgents(ctx, user, items)
	if err != nil {
		return nil, apperr.New(503, "agent catalog unavailable", err.Error())
	}
	resolver := s.newGatewayAddressResolver(ctx)
	result := make([]model.AgentInfo, 0, len(visible))
	for _, item := range visible {
		item.Info.HTTPURL = resolver.resolveURL(buildPortalMcpHTTPPath(item.Info.McpServerName), buildPortalMcpHTTPPath(item.Info.McpServerName), false)
		item.Info.SSEURL = resolver.resolveURL(buildPortalMcpSSEPath(item.Info.McpServerName), buildPortalMcpSSEPath(item.Info.McpServerName), false)
		result = append(result, item.Info)
	}
	return result, nil
}

func (s *Service) GetAgentDetail(ctx context.Context, agentID string, user model.AuthUser) (model.AgentInfo, error) {
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedAgentID == "" {
		return model.AgentInfo{}, apperr.New(404, "agent not found")
	}
	record, err := s.db.GetOne(ctx, `
		SELECT
			agent_id,
			canonical_name,
			display_name,
			intro,
			description,
			icon_url,
			tags_json,
			mcp_server_name,
			tool_count,
			transport_types_json,
			resource_summary,
			prompt_summary,
			published_at,
			updated_at
		FROM portal_agent_catalog
		WHERE status = 'published'
		  AND agent_id = ?
		LIMIT 1`, normalizedAgentID)
	if err != nil {
		return model.AgentInfo{}, apperr.New(503, "agent catalog unavailable", err.Error())
	}
	if len(record) == 0 {
		return model.AgentInfo{}, apperr.New(404, "agent not found")
	}
	items := []publishedAgentRecord{{
		AgentID: normalizedAgentID,
		Info:    s.toPortalAgentInfo(record),
	}}
	visible, err := s.filterVisiblePublishedAgents(ctx, user, items)
	if err != nil {
		return model.AgentInfo{}, apperr.New(503, "agent catalog unavailable", err.Error())
	}
	if len(visible) == 0 {
		return model.AgentInfo{}, apperr.New(404, "agent not found")
	}
	resolver := s.newGatewayAddressResolver(ctx)
	info := visible[0].Info
	info.HTTPURL = resolver.resolveURL(buildPortalMcpHTTPPath(info.McpServerName), buildPortalMcpHTTPPath(info.McpServerName), false)
	info.SSEURL = resolver.resolveURL(buildPortalMcpSSEPath(info.McpServerName), buildPortalMcpSSEPath(info.McpServerName), false)
	return info, nil
}

func (s *Service) toPortalAgentInfo(record gdb.Record) model.AgentInfo {
	mcpServerName := strings.TrimSpace(record["mcp_server_name"].String())
	httpPath := buildPortalMcpHTTPPath(mcpServerName)
	ssePath := buildPortalMcpSSEPath(mcpServerName)
	return model.AgentInfo{
		ID:              strings.TrimSpace(record["agent_id"].String()),
		CanonicalName:   strings.TrimSpace(record["canonical_name"].String()),
		DisplayName:     strings.TrimSpace(record["display_name"].String()),
		Intro:           strings.TrimSpace(record["intro"].String()),
		Description:     strings.TrimSpace(record["description"].String()),
		IconURL:         strings.TrimSpace(record["icon_url"].String()),
		Tags:            parseJSONList(record["tags_json"].String()),
		McpServerName:   mcpServerName,
		ToolCount:       record["tool_count"].Int64(),
		TransportTypes:  parseJSONList(record["transport_types_json"].String()),
		ResourceSummary: strings.TrimSpace(record["resource_summary"].String()),
		PromptSummary:   strings.TrimSpace(record["prompt_summary"].String()),
		HTTPURL:         httpPath,
		SSEURL:          ssePath,
		PublishedAt:     model.NowText(record["published_at"].Time()),
		UpdatedAt:       model.NowText(record["updated_at"].Time()),
	}
}

func (s *Service) filterVisiblePublishedAgents(ctx context.Context, user model.AuthUser,
	items []publishedAgentRecord,
) ([]publishedAgentRecord, error) {
	if len(items) == 0 {
		return []publishedAgentRecord{}, nil
	}
	grantsByAgent, err := s.loadAgentGrants(ctx, items)
	if err != nil {
		return nil, err
	}
	ancestorIDs, err := s.listDepartmentAncestorIds(ctx, user.DepartmentID)
	if err != nil {
		return nil, err
	}
	ancestorSet := make(map[string]struct{}, len(ancestorIDs))
	for _, item := range ancestorIDs {
		if item != "" {
			ancestorSet[item] = struct{}{}
		}
	}
	consumerName := model.NormalizeUsername(user.ConsumerName)
	result := make([]publishedAgentRecord, 0, len(items))
	for _, item := range items {
		grants := grantsByAgent[item.AgentID]
		if len(grants) == 0 {
			result = append(result, item)
			continue
		}
		visible := false
		for _, grant := range grants {
			subjectType := strings.TrimSpace(grant["subject_type"].String())
			subjectID := strings.TrimSpace(grant["subject_id"].String())
			if subjectType == "consumer" && subjectID == consumerName {
				visible = true
				break
			}
			if subjectType == "department" {
				if _, ok := ancestorSet[subjectID]; ok {
					visible = true
					break
				}
			}
			if subjectType == "user_level" && userLevelRank(user.UserLevel) >= userLevelRank(subjectID) {
				visible = true
				break
			}
		}
		if visible {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Service) loadAgentGrants(ctx context.Context, items []publishedAgentRecord) (map[string]gdb.Result, error) {
	agentIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.AgentID != "" {
			agentIDs = append(agentIDs, item.AgentID)
		}
	}
	query, args := buildStringInQuery(`
		SELECT asset_id, subject_type, subject_id
		FROM asset_grant
		WHERE asset_type = 'agent_catalog'
		  AND asset_id IN (%s)`, agentIDs)
	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query agent grants failed")
	}
	result := make(map[string]gdb.Result, len(agentIDs))
	for _, record := range records {
		assetID := strings.TrimSpace(record["asset_id"].String())
		result[assetID] = append(result[assetID], record)
	}
	return result, nil
}
