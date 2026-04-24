package portal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
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

	now := model.NowInAppLocation()
	inviteRecord, err := s.db.GetOne(ctx, `
		SELECT invite_code FROM portal_invite_code
		WHERE invite_code = ? AND status = 'active' AND expires_at > ?`, strings.TrimSpace(req.InviteCode), now)
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
		PasswordHash: passwordHash,
		Status:       consts.UserStatusDisabled,
		Source:       "portal",
	}).Insert(); err != nil {
		return model.RegisterResult{}, gerror.Wrap(err, "create user failed")
	}
	if err = s.ensureMembershipForConsumer(ctx, username); err != nil {
		return model.RegisterResult{}, err
	}

	return model.RegisterResult{
		User: model.AuthUser{
			ConsumerName: username,
			DisplayName:  displayName,
			Email:        strings.TrimSpace(req.Email),
			UserLevel:    consts.UserLevelNormal,
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
	if user == nil {
		return model.AuthUser{}, apperr.New(401, "incorrect username or password")
	}
	matched, needsUpgrade := verifyPassword(user.PasswordHash, req.Password)
	if !matched {
		return model.AuthUser{}, apperr.New(401, "incorrect username or password")
	}
	if isPortalLoginBlockedUser(user.ConsumerName, user.Source) {
		return model.AuthUser{}, apperr.New(403, "account is not allowed to login portal")
	}
	if user.Status != consts.UserStatusActive {
		return model.AuthUser{}, apperr.New(403, "account disabled")
	}

	updateData := do.PortalUser{
		LastLoginAt: gtime.Now(),
	}
	if needsUpgrade {
		passwordHash, hashErr := hashPassword(req.Password)
		if hashErr != nil {
			return model.AuthUser{}, gerror.Wrap(hashErr, "upgrade password hash failed")
		}
		updateData.PasswordHash = passwordHash
	}
	if _, err = s.db.Model("portal_user").Ctx(ctx).Where("consumer_name", username).Data(updateData).Update(); err != nil {
		return model.AuthUser{}, gerror.Wrap(err, "update last login failed")
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

func (s *Service) ChangePassword(ctx context.Context, consumerName string, req model.ChangePasswordRequest) error {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	if normalizedConsumer == "" {
		return apperr.New(401, "unauthorized")
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		return apperr.New(400, "oldPassword and newPassword are required")
	}
	if len(req.NewPassword) < 8 {
		return apperr.New(400, "new password must be at least 8 characters")
	}
	if req.OldPassword == req.NewPassword {
		return apperr.New(400, "new password must be different from current password")
	}

	user, err := s.getUserByName(ctx, normalizedConsumer)
	if err != nil {
		return err
	}
	if user == nil {
		return apperr.New(404, "user not found")
	}
	if matched, _ := verifyPassword(user.PasswordHash, req.OldPassword); !matched {
		return apperr.New(400, "current password is incorrect")
	}

	passwordHash, err := hashPassword(req.NewPassword)
	if err != nil {
		return gerror.Wrap(err, "encrypt password failed")
	}

	if err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Model("portal_user").Ctx(ctx).Where("consumer_name", normalizedConsumer).Data(do.PortalUser{
			PasswordHash: passwordHash,
		}).Update(); txErr != nil {
			return gerror.Wrap(txErr, "update user password failed")
		}
		if _, txErr := tx.Model("portal_session").Ctx(ctx).Where("consumer_name", normalizedConsumer).Delete(); txErr != nil {
			return gerror.Wrap(txErr, "clear user sessions failed")
		}
		return nil
	}); err != nil {
		return gerror.Wrap(err, "change password failed")
	}

	return nil
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
		SELECT u.consumer_name, u.display_name, u.email, u.user_level, u.status, u.source
		FROM portal_session s
		JOIN portal_user u ON u.consumer_name = s.consumer_name
		WHERE s.session_token = ? AND s.expires_at > ? AND COALESCE(u.is_deleted, FALSE) = FALSE`, token, model.NowInAppLocation())
	if err != nil {
		return model.AuthUser{}, gerror.Wrap(err, "query session failed")
	}
	if record.IsEmpty() {
		return model.AuthUser{}, apperr.New(401, "unauthorized")
	}
	if isPortalLoginBlockedUser(record["consumer_name"].String(), record["source"].String()) {
		return model.AuthUser{}, apperr.New(403, "account is not allowed to login portal")
	}

	orgContext, err := s.loadUserOrgContext(ctx, record["consumer_name"].String())
	if err != nil {
		return model.AuthUser{}, err
	}
	user := model.AuthUser{
		ConsumerName:      record["consumer_name"].String(),
		DisplayName:       record["display_name"].String(),
		Email:             record["email"].String(),
		DepartmentID:      orgContext.DepartmentID,
		DepartmentName:    orgContext.DepartmentName,
		DepartmentPath:    orgContext.DepartmentPath,
		AdminConsumerName: orgContext.AdminConsumerName,
		IsDepartmentAdmin: orgContext.IsDepartmentAdmin,
		UserLevel:         normalizeUserLevel(record["user_level"].String()),
		Status:            record["status"].String(),
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
		SELECT u.consumer_name, u.display_name, u.email, m.department_id,
			u.user_level, u.status, u.source, u.password_hash, u.last_login_at
		FROM portal_user u
		LEFT JOIN org_account_membership m ON m.consumer_name = u.consumer_name
		WHERE u.consumer_name = ? AND COALESCE(u.is_deleted, FALSE) = FALSE`, consumerName)
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
	matched, _ := verifyPassword(hash, raw)
	return matched
}

func verifyPassword(hash string, raw string) (bool, bool) {
	trimmedHash := strings.TrimSpace(hash)
	if trimmedHash == "" || raw == "" {
		return false, false
	}
	if bcrypt.CompareHashAndPassword([]byte(trimmedHash), []byte(raw)) == nil {
		return true, false
	}
	if strings.EqualFold(trimmedHash, legacyPasswordHash(raw)) {
		return true, true
	}
	return false, false
}

func legacyPasswordHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeUserLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case consts.UserLevelNormal, consts.UserLevelPlus, consts.UserLevelPro, consts.UserLevelUltra:
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return consts.UserLevelNormal
	}
}
