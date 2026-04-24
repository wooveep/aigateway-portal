package portal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"golang.org/x/oauth2"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
	portalshared "higress-portal-backend/schema/shared"
)

const (
	portalSSOProviderTypeOIDC = "oidc"
	portalSSOProviderKey      = "portal-oidc"
	portalSSOStateCookieTTL   = 10 * time.Minute
)

type portalSSOConfigRecord struct {
	Enabled               bool
	ProviderType          string
	DisplayName           string
	IssuerURL             string
	ClientID              string
	ClientSecretEncrypted string
	Scopes                []string
	ClaimMapping          portalSSOClaimMapping
	UpdatedBy             string
}

type portalSSOClaimMapping struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
}

type portalSSOStatePayload struct {
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"codeVerifier"`
	RedirectPath string `json:"redirectPath"`
	ExpireAt     int64  `json:"expireAt"`
}

type portalSSOLoginResult struct {
	User           *model.AuthUser
	RedirectPath   string
	PendingMessage string
}

type portalSSOIdentityRecord struct {
	ProviderKey  string
	Issuer       string
	Subject      string
	ConsumerName string
	Email        string
	DisplayName  string
}

type portalLocalUserRecord struct {
	model.PortalUserRow
	IsDeleted bool `orm:"is_deleted"`
}

func (s *Service) GetPublicSSOConfig(ctx context.Context) (model.PublicSSOConfig, error) {
	cfg, err := s.loadPortalSSOConfig(ctx)
	if err != nil {
		return model.PublicSSOConfig{}, err
	}
	return model.PublicSSOConfig{
		Enabled:     cfg.Enabled,
		DisplayName: cfg.DisplayName,
	}, nil
}

func (s *Service) BuildSSOAuthorizeURL(ctx context.Context, callbackURL, redirectPath string) (string, string, error) {
	cfg, err := s.loadEnabledPortalSSOConfig(ctx)
	if err != nil {
		return "", "", err
	}

	oauthConfig, _, err := s.buildPortalSSOOAuthConfig(ctx, cfg, callbackURL)
	if err != nil {
		return "", "", err
	}

	payload := portalSSOStatePayload{
		State:        randomString(32),
		Nonce:        randomString(32),
		CodeVerifier: randomString(64),
		RedirectPath: sanitizePortalRedirectPath(redirectPath),
		ExpireAt:     time.Now().Add(portalSSOStateCookieTTL).Unix(),
	}
	stateCookieValue, err := s.encodePortalSSOStateCookie(payload)
	if err != nil {
		return "", "", gerror.Wrap(err, "encode portal sso state failed")
	}

	authURL := oauthConfig.AuthCodeURL(
		payload.State,
		oidc.Nonce(payload.Nonce),
		oauth2.SetAuthURLParam("code_challenge", portalSSOCodeChallenge(payload.CodeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return authURL, stateCookieValue, nil
}

func (s *Service) CompleteSSOLogin(ctx context.Context, callbackURL, returnedState, code, stateCookieValue string) (portalSSOLoginResult, error) {
	cfg, err := s.loadEnabledPortalSSOConfig(ctx)
	if err != nil {
		return portalSSOLoginResult{}, err
	}
	if strings.TrimSpace(code) == "" {
		return portalSSOLoginResult{}, apperr.New(400, "missing oidc authorization code")
	}

	statePayload, err := s.decodePortalSSOStateCookie(stateCookieValue)
	if err != nil {
		return portalSSOLoginResult{}, apperr.New(400, "invalid sso login state")
	}
	if statePayload.State == "" || statePayload.State != strings.TrimSpace(returnedState) {
		return portalSSOLoginResult{}, apperr.New(400, "invalid sso login state")
	}
	if statePayload.ExpireAt <= time.Now().Unix() {
		return portalSSOLoginResult{}, apperr.New(400, "sso login state expired")
	}

	oauthConfig, provider, err := s.buildPortalSSOOAuthConfig(ctx, cfg, callbackURL)
	if err != nil {
		return portalSSOLoginResult{}, err
	}

	token, err := oauthConfig.Exchange(
		s.portalSSOHTTPContext(ctx),
		code,
		oauth2.SetAuthURLParam("code_verifier", statePayload.CodeVerifier),
	)
	if err != nil {
		return portalSSOLoginResult{}, apperr.New(400, "oidc token exchange failed")
	}

	rawIDToken, _ := token.Extra("id_token").(string)
	if strings.TrimSpace(rawIDToken) == "" {
		return portalSSOLoginResult{}, apperr.New(400, "oidc provider did not return id_token")
	}

	idToken, err := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}).Verify(s.portalSSOHTTPContext(ctx), rawIDToken)
	if err != nil {
		return portalSSOLoginResult{}, apperr.New(400, "oidc id_token verification failed")
	}

	claims := make(map[string]any)
	if err := idToken.Claims(&claims); err != nil {
		return portalSSOLoginResult{}, apperr.New(400, "oidc id_token claims are invalid")
	}
	if nonce := strings.TrimSpace(claimString(claims, "nonce")); nonce == "" || nonce != statePayload.Nonce {
		return portalSSOLoginResult{}, apperr.New(400, "oidc nonce verification failed")
	}

	result, err := s.resolvePortalSSOLogin(ctx, cfg, idToken, claims, statePayload.RedirectPath)
	if err != nil {
		return portalSSOLoginResult{}, err
	}
	return result, nil
}

func (s *Service) ResolveSSORedirectPath(stateCookieValue string) string {
	payload, err := s.decodePortalSSOStateCookie(stateCookieValue)
	if err != nil {
		return "/billing"
	}
	return sanitizePortalRedirectPath(payload.RedirectPath)
}

func (s *Service) resolvePortalSSOLogin(ctx context.Context, cfg portalSSOConfigRecord, idToken *oidc.IDToken,
	claims map[string]any, redirectPath string,
) (portalSSOLoginResult, error) {
	subject := strings.TrimSpace(idToken.Subject)
	if subject == "" {
		return portalSSOLoginResult{}, apperr.New(400, "oidc identity subject is missing")
	}

	var (
		email       = strings.ToLower(strings.TrimSpace(mappedClaimString(claims, cfg.ClaimMapping.Email)))
		displayName = strings.TrimSpace(mappedClaimString(claims, cfg.ClaimMapping.DisplayName))
	)
	if displayName == "" {
		displayName = emailLocalPart(email)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(mappedClaimString(claims, cfg.ClaimMapping.Username))
	}

	claimsJSON, err := marshalPortalSSOClaims(claims)
	if err != nil {
		return portalSSOLoginResult{}, gerror.Wrap(err, "marshal portal sso claims failed")
	}
	emailVerified := claimBool(claims, "email_verified")

	identity, err := s.getPortalSSOIdentityBySubject(ctx, idToken.Issuer, subject)
	if err != nil {
		return portalSSOLoginResult{}, err
	}
	if identity != nil {
		user, userErr := s.getPortalLocalUserByName(ctx, identity.ConsumerName)
		if userErr != nil {
			return portalSSOLoginResult{}, userErr
		}
		switch {
		case user == nil:
			identity = nil
		case user.IsDeleted:
			if strings.EqualFold(strings.TrimSpace(user.Source), "sso") {
				return s.restoreDeletedPortalSSOLoginForConsumer(ctx, user, idToken.Issuer, subject, email, emailVerified, displayName, claimsJSON, redirectPath)
			}
			identity = nil
		default:
			return s.completePortalSSOLoginForConsumer(ctx, identity.ConsumerName, idToken.Issuer, subject, email, emailVerified, displayName, claimsJSON, redirectPath)
		}
	}
	if identity != nil {
		return s.completePortalSSOLoginForConsumer(ctx, identity.ConsumerName, idToken.Issuer, subject, email, emailVerified, displayName, claimsJSON, redirectPath)
	}

	if email == "" {
		return portalSSOLoginResult{}, apperr.New(400, "oidc identity email is required")
	}

	users, err := s.listUsersByEmail(ctx, email)
	if err != nil {
		return portalSSOLoginResult{}, err
	}
	switch len(users) {
	case 0:
		deletedUser, deletedErr := s.getDeletedPortalSSOUserByEmail(ctx, email)
		if deletedErr != nil {
			return portalSSOLoginResult{}, deletedErr
		}
		if deletedUser != nil {
			return s.restoreDeletedPortalSSOLoginForConsumer(ctx, deletedUser, idToken.Issuer, subject, email, emailVerified, displayName, claimsJSON, redirectPath)
		}
		consumerName, err := s.allocatePortalSSOConsumerName(ctx, email)
		if err != nil {
			return portalSSOLoginResult{}, err
		}
		if displayName == "" {
			displayName = consumerName
		}
		if err = s.createPortalSSOUserAndIdentity(ctx, idToken.Issuer, subject, consumerName, email, displayName, emailVerified, claimsJSON); err != nil {
			return portalSSOLoginResult{}, err
		}
		return portalSSOLoginResult{
			RedirectPath:   sanitizePortalRedirectPath(redirectPath),
			PendingMessage: "账号已创建，待管理员启用",
		}, nil
	case 1:
		return s.completePortalSSOLoginForConsumer(ctx, users[0].ConsumerName, idToken.Issuer, subject, email, emailVerified, displayName, claimsJSON, redirectPath)
	default:
		return portalSSOLoginResult{}, apperr.New(409, "multiple local accounts matched the same email, please contact administrator")
	}
}

func (s *Service) completePortalSSOLoginForConsumer(ctx context.Context, consumerName, issuer, subject, email string,
	emailVerified bool, displayName, claimsJSON, redirectPath string,
) (portalSSOLoginResult, error) {
	user, err := s.getUserByName(ctx, consumerName)
	if err != nil {
		return portalSSOLoginResult{}, err
	}
	if user == nil {
		return portalSSOLoginResult{}, apperr.New(404, "linked local account not found")
	}
	if isPortalLoginBlockedUser(user.ConsumerName, user.Source) {
		return portalSSOLoginResult{}, apperr.New(403, "account is not allowed to login portal")
	}

	now := model.NowInAppLocation()
	if err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(`
			INSERT INTO portal_user_sso_identity (
				provider_key, issuer, subject, consumer_name, email, email_verified, display_name, claims_json, linked_at, last_login_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`+s.upsertClause([]string{"provider_key", "issuer", "subject"},
			s.assignExcluded("consumer_name"),
			s.assignExcluded("email"),
			s.assignExcluded("email_verified"),
			s.assignExcluded("display_name"),
			s.assignExcluded("claims_json"),
			s.assignExcluded("last_login_at"),
			s.assignExcluded("updated_at"))+``,
			portalSSOProviderKey,
			issuer,
			subject,
			consumerName,
			email,
			emailVerified,
			displayName,
			claimsJSON,
			now,
			now,
			now,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "upsert portal sso identity failed")
		}
		if user.Status == consts.UserStatusActive {
			if _, txErr := tx.Exec(`
				UPDATE portal_user
				SET last_login_at = ?, updated_at = CURRENT_TIMESTAMP
				WHERE consumer_name = ?`,
				now,
				consumerName,
			); txErr != nil {
				return gerror.Wrap(txErr, "update portal user last_login_at failed")
			}
		}
		return nil
	}); err != nil {
		return portalSSOLoginResult{}, err
	}

	if user.Status != consts.UserStatusActive {
		return portalSSOLoginResult{
			RedirectPath:   sanitizePortalRedirectPath(redirectPath),
			PendingMessage: "账号待管理员启用",
		}, nil
	}

	authUser, err := s.loadAuthUserByConsumer(ctx, consumerName)
	if err != nil {
		return portalSSOLoginResult{}, err
	}
	return portalSSOLoginResult{
		User:         &authUser,
		RedirectPath: sanitizePortalRedirectPath(redirectPath),
	}, nil
}

func (s *Service) restoreDeletedPortalSSOLoginForConsumer(ctx context.Context, user *portalLocalUserRecord,
	issuer, subject, email string, emailVerified bool, displayName, claimsJSON, redirectPath string,
) (portalSSOLoginResult, error) {
	if user == nil {
		return portalSSOLoginResult{}, apperr.New(404, "linked local account not found")
	}

	restoredDisplayName := strings.TrimSpace(displayName)
	if restoredDisplayName == "" {
		restoredDisplayName = strings.TrimSpace(user.DisplayName)
	}
	if restoredDisplayName == "" {
		restoredDisplayName = emailLocalPart(email)
	}
	if restoredDisplayName == "" {
		restoredDisplayName = strings.TrimSpace(user.ConsumerName)
	}

	restoredEmail := strings.ToLower(strings.TrimSpace(email))
	if restoredEmail == "" {
		restoredEmail = strings.ToLower(strings.TrimSpace(user.Email))
	}

	if _, err := s.db.Exec(ctx, `
		UPDATE portal_user
		SET display_name = ?,
			email = ?,
			is_deleted = FALSE,
			deleted_at = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE consumer_name = ?`,
		restoredDisplayName,
		restoredEmail,
		user.ConsumerName,
	); err != nil {
		return portalSSOLoginResult{}, gerror.Wrap(err, "restore deleted portal sso user failed")
	}
	if err := s.ensureMembershipForConsumer(ctx, user.ConsumerName); err != nil {
		return portalSSOLoginResult{}, err
	}
	return s.completePortalSSOLoginForConsumer(ctx, user.ConsumerName, issuer, subject, restoredEmail, emailVerified, restoredDisplayName, claimsJSON, redirectPath)
}

func (s *Service) createPortalSSOUserAndIdentity(ctx context.Context, issuer, subject, consumerName, email,
	displayName string, emailVerified bool, claimsJSON string,
) error {
	passwordHash, err := hashPassword("sso:" + randomString(32))
	if err != nil {
		return gerror.Wrap(err, "hash sso placeholder password failed")
	}

	now := model.NowInAppLocation()
	if err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(`
			INSERT INTO portal_user (
				consumer_name, display_name, email, password_hash, status, source, user_level, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			consumerName,
			displayName,
			email,
			passwordHash,
			consts.UserStatusPending,
			"sso",
			consts.UserLevelNormal,
			now,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert portal sso user failed")
		}
		if _, txErr := tx.Exec(`
			INSERT INTO org_account_membership (consumer_name, department_id, parent_consumer_name, created_at, updated_at)
			VALUES (?, NULL, NULL, ?, ?)
			`+s.upsertClause([]string{"consumer_name"},
			s.assignExcluded("updated_at"))+``,
			consumerName,
			now,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert portal sso membership failed")
		}
		if _, txErr := tx.Exec(`
			INSERT INTO portal_user_sso_identity (
				provider_key, issuer, subject, consumer_name, email, email_verified, display_name, claims_json, linked_at, last_login_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
			portalSSOProviderKey,
			issuer,
			subject,
			consumerName,
			email,
			emailVerified,
			displayName,
			claimsJSON,
			now,
			now,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert portal sso identity failed")
		}
		return nil
	}); err != nil {
		return gerror.Wrap(err, "create portal sso user failed")
	}
	return nil
}

func (s *Service) loadAuthUserByConsumer(ctx context.Context, consumerName string) (model.AuthUser, error) {
	user, err := s.getUserByName(ctx, consumerName)
	if err != nil {
		return model.AuthUser{}, err
	}
	if user == nil {
		return model.AuthUser{}, apperr.New(404, "user not found")
	}

	orgContext, err := s.loadUserOrgContext(ctx, user.ConsumerName)
	if err != nil {
		return model.AuthUser{}, err
	}
	return model.AuthUser{
		ConsumerName:      user.ConsumerName,
		DisplayName:       user.DisplayName,
		Email:             user.Email,
		DepartmentID:      orgContext.DepartmentID,
		DepartmentName:    orgContext.DepartmentName,
		DepartmentPath:    orgContext.DepartmentPath,
		AdminConsumerName: orgContext.AdminConsumerName,
		IsDepartmentAdmin: orgContext.IsDepartmentAdmin,
		UserLevel:         normalizeUserLevel(user.UserLevel),
		Status:            user.Status,
	}, nil
}

func (s *Service) getPortalSSOIdentityBySubject(ctx context.Context, issuer, subject string) (*portalSSOIdentityRecord, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT provider_key, issuer, subject, consumer_name, email, display_name
		FROM portal_user_sso_identity
		WHERE provider_key = ? AND issuer = ? AND subject = ?
		LIMIT 1`,
		portalSSOProviderKey,
		strings.TrimSpace(issuer),
		strings.TrimSpace(subject),
	)
	if err != nil {
		return nil, gerror.Wrap(err, "query portal sso identity failed")
	}
	if record.IsEmpty() {
		return nil, nil
	}
	return &portalSSOIdentityRecord{
		ProviderKey:  record["provider_key"].String(),
		Issuer:       record["issuer"].String(),
		Subject:      record["subject"].String(),
		ConsumerName: record["consumer_name"].String(),
		Email:        record["email"].String(),
		DisplayName:  record["display_name"].String(),
	}, nil
}

func (s *Service) listUsersByEmail(ctx context.Context, email string) ([]model.PortalUserRow, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT u.consumer_name, u.display_name, u.email, m.department_id,
			u.user_level, u.status, u.source, u.password_hash, u.last_login_at
		FROM portal_user u
		LEFT JOIN org_account_membership m ON m.consumer_name = u.consumer_name
		WHERE LOWER(u.email) = LOWER(?)
		  AND COALESCE(u.is_deleted, FALSE) = FALSE
		ORDER BY u.consumer_name ASC`, strings.TrimSpace(email))
	if err != nil {
		return nil, gerror.Wrap(err, "query portal users by email failed")
	}
	items := make([]model.PortalUserRow, 0, len(records))
	for _, record := range records {
		var item model.PortalUserRow
		if err := record.Struct(&item); err != nil {
			return nil, gerror.Wrap(err, "convert portal user by email failed")
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) getDeletedPortalSSOUserByEmail(ctx context.Context, email string) (*portalLocalUserRecord, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT u.consumer_name, u.display_name, u.email, m.department_id,
			u.user_level, u.status, u.source, u.password_hash, u.last_login_at, u.is_deleted
		FROM portal_user u
		LEFT JOIN org_account_membership m ON m.consumer_name = u.consumer_name
		WHERE LOWER(u.email) = LOWER(?)
		  AND COALESCE(u.is_deleted, FALSE) = TRUE
		  AND LOWER(u.source) = 'sso'
		ORDER BY u.updated_at DESC NULLS LAST, u.consumer_name ASC
		LIMIT 1`, strings.TrimSpace(email))
	if err != nil {
		return nil, gerror.Wrap(err, "query deleted portal sso user by email failed")
	}
	if record.IsEmpty() {
		return nil, nil
	}
	var item portalLocalUserRecord
	if err := record.Struct(&item); err != nil {
		return nil, gerror.Wrap(err, "convert deleted portal sso user by email failed")
	}
	return &item, nil
}

func (s *Service) getPortalLocalUserByName(ctx context.Context, consumerName string) (*portalLocalUserRecord, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT u.consumer_name, u.display_name, u.email, m.department_id,
			u.user_level, u.status, u.source, u.password_hash, u.last_login_at, u.is_deleted
		FROM portal_user u
		LEFT JOIN org_account_membership m ON m.consumer_name = u.consumer_name
		WHERE u.consumer_name = ?
		LIMIT 1`, strings.TrimSpace(consumerName))
	if err != nil {
		return nil, gerror.Wrap(err, "query portal local user failed")
	}
	if record.IsEmpty() {
		return nil, nil
	}
	var item portalLocalUserRecord
	if err := record.Struct(&item); err != nil {
		return nil, gerror.Wrap(err, "convert portal local user failed")
	}
	return &item, nil
}

func (s *Service) allocatePortalSSOConsumerName(ctx context.Context, email string) (string, error) {
	base := model.NormalizeUsername(emailLocalPart(email))
	if base == "" {
		base = "sso-user"
	}

	candidates := []string{
		base,
		fmt.Sprintf("%s-%s", base, sha256Hex(email)[:6]),
	}
	for _, candidate := range candidates {
		existing, err := s.getPortalLocalUserByName(ctx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	for i := 0; i < 8; i++ {
		candidate := fmt.Sprintf("%s-%s", base, strings.ToLower(randomString(4)))
		existing, err := s.getPortalLocalUserByName(ctx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", apperr.New(500, "failed to allocate local username for sso account")
}

func (s *Service) loadPortalSSOConfig(ctx context.Context) (portalSSOConfigRecord, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT enabled, provider_type, display_name, issuer_url, client_id, client_secret_encrypted,
			scopes_json, claim_mapping_json, updated_by
		FROM portal_sso_config
		WHERE provider_type = ?
		LIMIT 1`, portalSSOProviderTypeOIDC)
	if err != nil {
		return portalSSOConfigRecord{}, gerror.Wrap(err, "query portal sso config failed")
	}

	cfg := portalSSOConfigRecord{
		ProviderType: portalSSOProviderTypeOIDC,
		DisplayName:  "企业 SSO 登录",
		Scopes:       []string{"openid", "profile", "email"},
		ClaimMapping: defaultPortalSSOClaimMapping(),
	}
	if record.IsEmpty() {
		return cfg, nil
	}

	cfg.Enabled = record["enabled"].Bool()
	cfg.ProviderType = firstNonEmptyNonBlank(record["provider_type"].String(), portalSSOProviderTypeOIDC)
	cfg.DisplayName = firstNonEmptyNonBlank(record["display_name"].String(), cfg.DisplayName)
	cfg.IssuerURL = strings.TrimSpace(record["issuer_url"].String())
	cfg.ClientID = strings.TrimSpace(record["client_id"].String())
	cfg.ClientSecretEncrypted = strings.TrimSpace(record["client_secret_encrypted"].String())
	cfg.UpdatedBy = strings.TrimSpace(record["updated_by"].String())

	if scopes := parseStringJSONArray(record["scopes_json"].String()); len(scopes) > 0 {
		cfg.Scopes = scopes
	}
	if mappingRaw := strings.TrimSpace(record["claim_mapping_json"].String()); mappingRaw != "" {
		var mapping portalSSOClaimMapping
		if err := json.Unmarshal([]byte(mappingRaw), &mapping); err == nil {
			cfg.ClaimMapping = normalizePortalSSOClaimMapping(mapping)
		}
	}
	return cfg, nil
}

func (s *Service) loadEnabledPortalSSOConfig(ctx context.Context) (portalSSOConfigRecord, error) {
	cfg, err := s.loadPortalSSOConfig(ctx)
	if err != nil {
		return portalSSOConfigRecord{}, err
	}
	if !cfg.Enabled {
		return portalSSOConfigRecord{}, apperr.New(404, "portal sso is disabled")
	}
	if strings.TrimSpace(cfg.IssuerURL) == "" || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecretEncrypted) == "" {
		return portalSSOConfigRecord{}, apperr.New(503, "portal sso is not configured")
	}
	return cfg, nil
}

func (s *Service) buildPortalSSOOAuthConfig(ctx context.Context, cfg portalSSOConfigRecord, callbackURL string) (*oauth2.Config, *oidc.Provider, error) {
	clientSecret, err := portalshared.DecryptPortalSSOSecret(cfg.ClientSecretEncrypted)
	if err != nil {
		return nil, nil, apperr.New(503, "portal sso secret is invalid")
	}
	if strings.TrimSpace(clientSecret) == "" {
		return nil, nil, apperr.New(503, "portal sso secret is missing")
	}

	provider, err := oidc.NewProvider(s.portalSSOHTTPContext(ctx), cfg.IssuerURL)
	if err != nil {
		return nil, nil, apperr.New(503, "portal sso discovery failed")
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  strings.TrimSpace(callbackURL),
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}, provider, nil
}

func (s *Service) portalSSOHTTPContext(ctx context.Context) context.Context {
	ctx = oidc.ClientContext(ctx, s.httpClient)
	return context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)
}

func (s *Service) encodePortalSSOStateCookie(payload portalSSOStatePayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	_, _ = mac.Write([]byte(encodedBody))
	return encodedBody + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) decodePortalSSOStateCookie(raw string) (portalSSOStatePayload, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 2 {
		return portalSSOStatePayload{}, errors.New("invalid sso state cookie")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return portalSSOStatePayload{}, err
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return portalSSOStatePayload{}, errors.New("invalid sso state cookie signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return portalSSOStatePayload{}, err
	}
	var payload portalSSOStatePayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return portalSSOStatePayload{}, err
	}
	payload.RedirectPath = sanitizePortalRedirectPath(payload.RedirectPath)
	return payload, nil
}

func portalSSOCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func defaultPortalSSOClaimMapping() portalSSOClaimMapping {
	return portalSSOClaimMapping{
		Email:       "email",
		DisplayName: "name",
		Username:    "preferred_username",
	}
}

func normalizePortalSSOClaimMapping(mapping portalSSOClaimMapping) portalSSOClaimMapping {
	defaults := defaultPortalSSOClaimMapping()
	return portalSSOClaimMapping{
		Email:       firstNonEmptyNonBlank(mapping.Email, defaults.Email),
		DisplayName: firstNonEmptyNonBlank(mapping.DisplayName, defaults.DisplayName),
		Username:    firstNonEmptyNonBlank(mapping.Username, defaults.Username),
	}
}

func marshalPortalSSOClaims(claims map[string]any) (string, error) {
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseStringJSONArray(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if normalized := strings.TrimSpace(item); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func mappedClaimString(claims map[string]any, key string) string {
	return claimString(claims, strings.TrimSpace(key))
}

func claimString(claims map[string]any, key string) string {
	if claims == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func claimBool(claims map[string]any, key string) bool {
	value, ok := claims[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func emailLocalPart(email string) string {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(trimmed, "@", 2)
	return model.NormalizeUsername(parts[0])
}

func sanitizePortalRedirectPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/billing"
	}
	if strings.HasPrefix(trimmed, "/login") || strings.HasPrefix(trimmed, "/register") {
		return "/billing"
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" {
		return "/billing"
	}
	return trimmed
}

func firstNonEmptyNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
