package middleware

import (
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/httpx"
	servicePortal "higress-portal-backend/internal/service/portal"
)

type Auth struct {
	svc *servicePortal.Service
}

func NewAuth(svc *servicePortal.Service) *Auth {
	return &Auth{svc: svc}
}

func (m *Auth) Handler(r *ghttp.Request) {
	cfg := m.svc.Config()
	token := strings.TrimSpace(r.Cookie.Get(cfg.SessionCookieName).String())
	if token == "" {
		httpx.WriteJSON(r, http.StatusUnauthorized, g.Map{"message": "unauthorized"})
		r.ExitAll()
		return
	}

	user, err := m.svc.AuthenticateSession(r.Context(), token)
	if err != nil {
		httpx.WriteError(r, err)
		r.ExitAll()
		return
	}

	r.SetCtxVar(consts.CtxUserKey, user)
	r.Middleware.Next()
}
