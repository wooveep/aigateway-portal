package portal

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
)

const revokedCredentialPrefix = "revoked-"

func (s *Service) StartKeyAuthSync(ctx context.Context) {
	if !s.cfg.KeyAuthSyncEnabled {
		return
	}
	if err := s.syncKeyAuthConsumers(ctx); err != nil {
		s.logf(ctx, "initial key-auth sync failed: %v", err)
	}

	interval := s.cfg.KeyAuthSyncInterval
	if interval < time.Second {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.syncKeyAuthConsumers(ctx); err != nil {
					s.logf(ctx, "key-auth sync failed: %v", err)
				}
			}
		}
	}()
}

type keyAuthCredential struct {
	KeyID      string
	Credential string
}

func (s *Service) syncKeyAuthConsumers(ctx context.Context) error {
	rows, err := s.db.GetAll(ctx, `
		SELECT consumer_name, status
		FROM portal_user
		WHERE COALESCE(is_deleted, FALSE) = FALSE
		ORDER BY consumer_name ASC`)
	if err != nil {
		return gerror.Wrap(err, "query portal users failed")
	}

	activeKeyRows, err := s.db.GetAll(ctx, `
		SELECT key_id, consumer_name, raw_key
		FROM portal_api_key
		WHERE status = 'active'
		  AND deleted_at IS NULL
		  AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY consumer_name ASC, id ASC`, model.NowInAppLocation())
	if err != nil {
		return gerror.Wrap(err, "query active api keys failed")
	}
	keyMap := make(map[string][]keyAuthCredential)
	for _, item := range activeKeyRows {
		consumerName := model.NormalizeUsername(item["consumer_name"].String())
		keyID := strings.TrimSpace(item["key_id"].String())
		rawKey := strings.TrimSpace(item["raw_key"].String())
		if consumerName == "" || rawKey == "" {
			continue
		}
		keyMap[consumerName] = appendKeyAuthCredentials(keyMap[consumerName], keyID, rawKey)
	}

	users := make([]string, 0, len(rows))
	statusMap := make(map[string]string, len(rows))
	for _, item := range rows {
		consumerName := model.NormalizeUsername(item["consumer_name"].String())
		if consumerName == "" {
			continue
		}
		users = append(users, consumerName)
		statusMap[consumerName] = strings.ToLower(strings.TrimSpace(item["status"].String()))
	}
	sort.Strings(users)

	consumers := make([]clientK8s.KeyAuthConsumer, 0, len(users))
	for _, consumerName := range users {
		keys := keyMap[consumerName]
		if statusMap[consumerName] == consts.UserStatusActive && len(keys) > 0 {
			added := false
			for _, key := range keys {
				if key.Credential == "" {
					continue
				}
				consumers = append(consumers, clientK8s.KeyAuthConsumer{
					Name:       consumerName,
					Credential: key.Credential,
					KeyID:      key.KeyID,
				})
				added = true
			}
			if added {
				continue
			}
		}
		consumers = append(consumers, clientK8s.KeyAuthConsumer{
			Name:       consumerName,
			Credential: revokedCredentialPrefix + sha256Hex("revoked:" + consumerName)[:32],
		})
	}

	if err = s.modelK8s.UpdateKeyAuthConsumers(ctx, consumers); err != nil {
		return gerror.Wrap(err, "update key-auth consumers failed")
	}
	return nil
}

func appendKeyAuthCredentials(existing []keyAuthCredential, keyID string, rawKey string) []keyAuthCredential {
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		signature := item.KeyID + "\x00" + item.Credential
		seen[signature] = struct{}{}
	}
	appendCredential := func(credential string) {
		credential = strings.TrimSpace(credential)
		if credential == "" {
			return
		}
		signature := keyID + "\x00" + credential
		if _, ok := seen[signature]; ok {
			return
		}
		seen[signature] = struct{}{}
		existing = append(existing, keyAuthCredential{
			KeyID:      keyID,
			Credential: credential,
		})
	}

	appendCredential(rawKey)
	appendCredential("Bearer " + rawKey)
	return existing
}
