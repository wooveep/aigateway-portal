package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
	"higress-portal-backend/internal/model/do"
)

func (s *Service) ListAPIKeys(ctx context.Context, consumerName string, includeRaw bool) ([]model.APIKeyRecord, error) {
	rows, err := s.listAPIKeyRows(ctx, consumerName)
	if err != nil {
		return nil, err
	}

	items := make([]model.APIKeyRecord, 0, len(rows))
	for _, item := range rows {
		lastUsed := "-"
		if item.LastUsedAt != nil {
			lastUsed = model.NowText(*item.LastUsedAt)
		}
		keyValue := item.Masked
		if includeRaw {
			keyValue = item.RawKey
		}
		items = append(items, model.APIKeyRecord{
			ID:         item.KeyID,
			Name:       item.Name,
			Key:        keyValue,
			Status:     item.Status,
			CreatedAt:  model.NowText(item.CreatedAt),
			LastUsed:   lastUsed,
			TotalCalls: item.TotalCalls,
		})
	}
	return items, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, user model.AuthUser, req model.CreateAPIKeyRequest) (model.APIKeyRecord, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.APIKeyRecord{}, apperr.New(400, "name is required")
	}

	rawKey := randomToken("hgpk_live_")
	keyID := fmt.Sprintf("KEY%d", time.Now().UnixMilli())
	if _, err := s.db.Model("portal_api_key").Ctx(ctx).Data(do.PortalApiKey{
		KeyId:        keyID,
		ConsumerName: user.ConsumerName,
		Name:         name,
		KeyMasked:    model.MaskKey(rawKey),
		KeyHash:      sha256Hex(rawKey),
		RawKey:       rawKey,
		Status:       consts.APIKeyStatusActive,
	}).Insert(); err != nil {
		return model.APIKeyRecord{}, gerror.Wrap(err, "create api key failed")
	}

	return model.APIKeyRecord{
		ID:         keyID,
		Name:       name,
		Key:        rawKey,
		Status:     consts.APIKeyStatusActive,
		CreatedAt:  model.NowText(time.Now()),
		LastUsed:   "-",
		TotalCalls: 0,
	}, nil
}

func (s *Service) UpdateAPIKeyStatus(ctx context.Context, user model.AuthUser, keyID string, status string) (model.APIKeyRecord, error) {
	if status != consts.APIKeyStatusActive && status != consts.APIKeyStatusDisabled {
		return model.APIKeyRecord{}, apperr.New(400, "status must be active or disabled")
	}

	result, err := s.db.Model("portal_api_key").Ctx(ctx).
		Where("key_id = ? AND consumer_name = ?", keyID, user.ConsumerName).
		Data(do.PortalApiKey{Status: status}).Update()
	if err != nil {
		return model.APIKeyRecord{}, gerror.Wrap(err, "update api key failed")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}

	record, err := s.db.GetOne(ctx, `
		SELECT key_id, name, key_masked, status, created_at, total_calls, last_used_at
		FROM portal_api_key
		WHERE key_id = ? AND consumer_name = ?`, keyID, user.ConsumerName)
	if err != nil {
		return model.APIKeyRecord{}, gerror.Wrap(err, "query api key failed")
	}
	if record.IsEmpty() {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}

	resp := model.APIKeyRecord{
		ID:         record["key_id"].String(),
		Name:       record["name"].String(),
		Key:        record["key_masked"].String(),
		Status:     record["status"].String(),
		CreatedAt:  model.NowText(record["created_at"].Time()),
		LastUsed:   "-",
		TotalCalls: record["total_calls"].Int64(),
	}
	if !record["last_used_at"].IsEmpty() {
		resp.LastUsed = model.NowText(record["last_used_at"].Time())
	}
	return resp, nil
}

func (s *Service) DeleteAPIKey(ctx context.Context, user model.AuthUser, keyID string) error {
	result, err := s.db.Model("portal_api_key").Ctx(ctx).Where("key_id = ? AND consumer_name = ?", keyID, user.ConsumerName).Delete()
	if err != nil {
		return gerror.Wrap(err, "delete api key failed")
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperr.New(404, "api key not found")
	}
	return nil
}

func (s *Service) listAPIKeyRows(ctx context.Context, consumerName string) ([]model.APIKeyRow, error) {
	var rows []model.APIKeyRow
	err := s.db.GetScan(ctx, &rows, `
		SELECT key_id, name, raw_key, key_masked, status, total_calls, last_used_at, created_at
		FROM portal_api_key
		WHERE consumer_name = ?
		ORDER BY created_at DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query api keys failed")
	}
	return rows, nil
}
