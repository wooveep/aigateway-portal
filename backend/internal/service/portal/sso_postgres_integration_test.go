//go:build integration
// +build integration

package portal

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/stretchr/testify/require"

	"higress-portal-backend/internal/config"
)

func TestResolvePortalSSOLoginRestoresDeletedAutoCreatedAccount(t *testing.T) {
	ctx := context.Background()
	dsn := startPortalCompatPostgres(t, ctx, "portal_sso_restore_pg_it")

	db, err := gdb.New(gdb.ConfigNode{Type: "pgsql", Link: gfPostgresLink(dsn)})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close(ctx))
	}()

	svc := &Service{
		cfg: config.Config{
			DBDriver: "postgres",
		},
		db: db,
	}
	require.NoError(t, svc.runMigrations(ctx))

	passwordHash, err := hashPassword("sso:placeholder")
	require.NoError(t, err)

	issuer := "http://keycloak.local/realms/master"
	subject := "subject-li-yuntian"
	email := "li.yuntian@ocloudware.com"

	_, err = db.Exec(ctx, `
		INSERT INTO portal_user (
			consumer_name, display_name, email, password_hash, status, source, user_level, is_deleted, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, TRUE, ?)`,
		"li.yuntian",
		"黎云天",
		email,
		passwordHash,
		"pending",
		"sso",
		"normal",
		time.Now().UTC(),
	)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `
		INSERT INTO portal_user_sso_identity (
			provider_key, issuer, subject, consumer_name, email, email_verified, display_name, claims_json, linked_at, last_login_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		portalSSOProviderKey,
		issuer,
		subject,
		"li.yuntian",
		email,
		true,
		"黎云天",
		`{"email":"li.yuntian@ocloudware.com"}`,
	)
	require.NoError(t, err)

	result, err := svc.resolvePortalSSOLogin(
		ctx,
		portalSSOConfigRecord{ClaimMapping: defaultPortalSSOClaimMapping()},
		&oidc.IDToken{Issuer: issuer, Subject: subject},
		map[string]any{
			"email":          email,
			"name":           "黎云天",
			"preferred_name": "liyuntian",
			"email_verified": true,
		},
		"/billing",
	)
	require.NoError(t, err)
	require.Equal(t, "账号待管理员启用", result.PendingMessage)

	record, err := db.GetOne(ctx, `
		SELECT consumer_name, is_deleted, deleted_at
		FROM portal_user
		WHERE consumer_name = ?`, "li.yuntian")
	require.NoError(t, err)
	require.False(t, record["is_deleted"].Bool())
	require.True(t, record["deleted_at"].IsEmpty())

	identity, err := db.GetOne(ctx, `
		SELECT consumer_name
		FROM portal_user_sso_identity
		WHERE provider_key = ? AND issuer = ? AND subject = ?`,
		portalSSOProviderKey,
		issuer,
		subject,
	)
	require.NoError(t, err)
	require.Equal(t, "li.yuntian", identity["consumer_name"].String())
}
