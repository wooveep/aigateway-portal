package portal

import (
	"net/http"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	servicePortal "higress-portal-backend/internal/service/portal"
)

func setSessionCookie(r *ghttp.Request, token string, svc *servicePortal.Service) {
	cfg := svc.Config()
	r.Cookie.SetCookie(
		cfg.SessionCookieName,
		token,
		"",
		"/",
		cfg.SessionTTL,
		ghttp.CookieOptions{
			SameSite: http.SameSiteLaxMode,
			Secure:   cfg.SessionSecureCookie,
			HttpOnly: true,
		},
	)
}

func clearSessionCookie(r *ghttp.Request, svc *servicePortal.Service) {
	cfg := svc.Config()
	r.Cookie.SetCookie(
		cfg.SessionCookieName,
		"",
		"",
		"/",
		-24*time.Hour,
		ghttp.CookieOptions{
			SameSite: http.SameSiteLaxMode,
			Secure:   cfg.SessionSecureCookie,
			HttpOnly: true,
		},
	)
}
