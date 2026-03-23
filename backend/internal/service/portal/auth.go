package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"
	"golang.org/x/crypto/bcrypt"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
	"higress-portal-backend/internal/model/do"
)

func (s *Service) Register(ctx context.Context, req model.RegisterRequest) (model.RegisterResult, error) {
	username := model.NormalizeUsername(req.Username)
	if username == "" || len(req.Password) < 8 || strings.TrimSpace(req.InviteCode) == "" {
		return model.RegisterResult{}, apperr.New(400, "inviteCode, username and password(>=8) are required")
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = username
	}

	inviteRecord, err := s.db.GetOne(ctx, `
		SELECT invite_code FROM portal_invite_code
		WHERE invite_code = ? AND status = 'active' AND expires_at > NOW()`, strings.TrimSpace(req.InviteCode))
	if err != nil {
		return model.RegisterResult{}, gerror.Wrap(err, "query invite code failed")
	}
	if inviteRecord.IsEmpty() {
		return model.RegisterResult{}, apperr.New(400, "invite code invalid or expired")
	}

	existing, err := s.getUserByName(ctx, username)
	if err != nil {
		return model.RegisterResult{}, err
	}
	if existing != nil {
		return model.RegisterResult{}, apperr.New(409, "username already exists")
	}

	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return model.RegisterResult{}, gerror.Wrap(err, "encrypt password failed")
	}

	if _, err = s.db.Model("portal_user").Ctx(ctx).Data(do.PortalUser{
		ConsumerName: username,
		DisplayName:  displayName,
		Email:        strings.TrimSpace(req.Email),
		Department:   strings.TrimSpace(req.Department),
		PasswordHash: passwordHash,
		Status:       consts.UserStatusDisabled,
		Source:       "portal",
	}).Insert(); err != nil {
		return model.RegisterResult{}, gerror.Wrap(err, "create user failed")
	}

	return model.RegisterResult{
		User: model.AuthUser{
			ConsumerName: username,
			DisplayName:  displayName,
			Email:        strings.TrimSpace(req.Email),
			Department:   strings.TrimSpace(req.Department),
			Status:       consts.UserStatusDisabled,
		},
		DefaultAPIKey: "",
	}, nil
}

func (s *Service) Login(ctx context.Context, req model.LoginRequest) (model.AuthUser, error) {
	username := model.NormalizeUsername(req.Username)
	if username == "" || req.Password == "" {
		return model.AuthUser{}, apperr.New(400, "username and password are required")
	}

	user, err := s.getUserByName(ctx, username)
	if err != nil {
		return model.AuthUser{}, err
	}
	if user == nil || !comparePassword(user.PasswordHash, req.Password) {
		return model.AuthUser{}, apperr.New(401, "incorrect username or password")
	}
	if isPortalLoginBlockedUser(user.ConsumerName, user.Source) {
		return model.AuthUser{}, apperr.New(403, "account is not allowed to login portal")
	}
	if user.Status != consts.UserStatusActive {
		return model.AuthUser{}, apperr.New(403, "account disabled")
	}

	if _, err = s.db.Model("portal_user").Ctx(ctx).Where("consumer_name", username).Data(do.PortalUser{
		LastLoginAt: gtime.Now(),
	}).Update(); err != nil {
		return model.AuthUser{}, gerror.Wrap(err, "update last login failed")
	}

	return model.AuthUser{
		ConsumerName: user.ConsumerName,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
		Department:   user.Department,
		Status:       user.Status,
	}, nil
}

func (s *Service) CreateSession(ctx context.Context, consumerName string) (string, error) {
	token := "sess_" + sha256Hex(fmt.Sprintf("%s:%s:%d:%s",
		consumerName,
		randomString(24),
		time.Now().UnixNano(),
		s.cfg.SessionSecret,
	))[:48]
	expireAt := time.Now().Add(s.cfg.SessionTTL)
	if _, err := s.db.Model("portal_session").Ctx(ctx).Data(do.PortalSession{
		SessionToken: token,
		ConsumerName: consumerName,
		ExpiresAt:    gtime.New(expireAt),
	}).Insert(); err != nil {
		return "", gerror.Wrap(err, "create session failed")
	}
	return token, nil
}

func (s *Service) ClearSession(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	if _, err := s.db.Model("portal_session").Ctx(ctx).Where("session_token", token).Delete(); err != nil {
		return gerror.Wrap(err, "clear session failed")
	}
	return nil
}

func (s *Service) AuthenticateSession(ctx context.Context, token string) (model.AuthUser, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT u.consumer_name, u.display_name, u.email, u.department, u.status, u.source
		FROM portal_session s
		JOIN portal_user u ON u.consumer_name = s.consumer_name
		WHERE s.session_token = ? AND s.expires_at > NOW()`, token)
	if err != nil {
		return model.AuthUser{}, gerror.Wrap(err, "query session failed")
	}
	if record.IsEmpty() {
		return model.AuthUser{}, apperr.New(401, "unauthorized")
	}
	if isPortalLoginBlockedUser(record["consumer_name"].String(), record["source"].String()) {
		return model.AuthUser{}, apperr.New(403, "account is not allowed to login portal")
	}

	user := model.AuthUser{
		ConsumerName: record["consumer_name"].String(),
		DisplayName:  record["display_name"].String(),
		Email:        record["email"].String(),
		Department:   record["department"].String(),
		Status:       record["status"].String(),
	}
	if user.Status != consts.UserStatusActive {
		return model.AuthUser{}, apperr.New(403, "account disabled")
	}

	_, _ = s.db.Model("portal_session").Ctx(ctx).Where("session_token", token).Data(do.PortalSession{
		LastSeenAt: gtime.Now(),
	}).Update()
	return user, nil
}

func isPortalLoginBlockedUser(consumerName string, source string) bool {
	name := strings.ToLower(strings.TrimSpace(consumerName))
	if name != "administrator" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(source), "system") {
		return true
	}
	return true
}

func (s *Service) getUserByName(ctx context.Context, consumerName string) (*model.PortalUserRow, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT consumer_name, display_name, email, department, status, source, password_hash, last_login_at
		FROM portal_user WHERE consumer_name = ?`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query user failed")
	}
	if record.IsEmpty() {
		return nil, nil
	}
	var user model.PortalUserRow
	if err = record.Struct(&user); err != nil {
		return nil, gerror.Wrap(err, "convert user failed")
	}
	return &user, nil
}

func hashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func comparePassword(hash string, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}
