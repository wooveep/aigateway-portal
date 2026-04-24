package portal

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	"higress-portal-backend/internal/httpx"
	servicePortal "higress-portal-backend/internal/service/portal"
)

const portalSSOStateCookieSuffix = "_oidc_state"

func (c *Controller) SSOConfig(r *ghttp.Request) {
	config, err := c.svc.GetPublicSSOConfig(r.Context())
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, config)
}

func (c *Controller) SSOAuthorize(r *ghttp.Request) {
	callbackURL := resolvePortalSSOCallbackURL(r, c.svc)
	redirectPath := sanitizePortalRedirectQuery(r.GetQuery("redirect").String())

	authURL, stateCookieValue, err := c.svc.BuildSSOAuthorizeURL(r.Context(), callbackURL, redirectPath)
	if err != nil {
		redirectPortalSSOToLogin(r, redirectPath, "", ssoRedirectMessage(err))
		return
	}

	setPortalSSOStateCookie(r, stateCookieValue, c.svc)
	redirectPortalRequest(r, authURL)
}

func (c *Controller) SSOCallback(r *ghttp.Request) {
	stateCookieValue := strings.TrimSpace(r.Cookie.Get(portalSSOStateCookieName(c.svc)).String())
	redirectPath := c.svc.ResolveSSORedirectPath(stateCookieValue)
	clearPortalSSOStateCookie(r, c.svc)

	if providerError := strings.TrimSpace(r.GetQuery("error").String()); providerError != "" {
		message := firstNonEmptyNonBlank(
			strings.TrimSpace(r.GetQuery("error_description").String()),
			providerError,
		)
		redirectPortalSSOToLogin(r, redirectPath, "", message)
		return
	}

	callbackURL := resolvePortalSSOCallbackURL(r, c.svc)
	result, err := c.svc.CompleteSSOLogin(
		r.Context(),
		callbackURL,
		r.GetQuery("state").String(),
		r.GetQuery("code").String(),
		stateCookieValue,
	)
	if err != nil {
		redirectPortalSSOToLogin(r, redirectPath, "", ssoRedirectMessage(err))
		return
	}

	if result.User == nil {
		redirectPortalSSOToLogin(r, result.RedirectPath, result.PendingMessage, "")
		return
	}

	token, err := c.svc.CreateSession(r.Context(), result.User.ConsumerName)
	if err != nil {
		redirectPortalSSOToLogin(r, result.RedirectPath, "", ssoRedirectMessage(err))
		return
	}
	setSessionCookie(r, token, c.svc)
	redirectPortalRequest(r, result.RedirectPath)
}

func resolvePortalSSOCallbackURL(r *ghttp.Request, svc *servicePortal.Service) string {
	baseURL := strings.TrimSpace(svc.Config().PortalPublicBaseURL)
	if baseURL == "" {
		baseURL = requestPublicBaseURL(r)
	}
	if parsed, err := url.Parse(baseURL); err == nil {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/auth/sso/callback"
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	return requestPublicBaseURL(r) + "/api/auth/sso/callback"
}

func requestPublicBaseURL(r *ghttp.Request) string {
	scheme := firstNonEmptyNonBlank(
		r.Header.Get("X-Forwarded-Proto"),
	)
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := firstNonEmptyNonBlank(
		r.Header.Get("X-Forwarded-Host"),
		r.Host,
	)
	return scheme + "://" + host
}

func redirectPortalSSOToLogin(r *ghttp.Request, redirectPath, message, errMessage string) {
	query := url.Values{}
	if sanitizedRedirect := sanitizePortalRedirectQuery(redirectPath); sanitizedRedirect != "/billing" {
		query.Set("redirect", sanitizedRedirect)
	}
	if strings.TrimSpace(message) != "" {
		query.Set("ssoMessage", strings.TrimSpace(message))
	}
	if strings.TrimSpace(errMessage) != "" {
		query.Set("ssoError", strings.TrimSpace(errMessage))
	}

	target := "/login"
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	redirectPortalRequest(r, target)
}

func redirectPortalRequest(r *ghttp.Request, target string) {
	r.Response.Header().Set("Location", target)
	r.Response.WriteStatusExit(http.StatusFound)
}

func setPortalSSOStateCookie(r *ghttp.Request, value string, svc *servicePortal.Service) {
	cfg := svc.Config()
	r.Cookie.SetCookie(
		portalSSOStateCookieName(svc),
		value,
		"",
		"/",
		10*time.Minute,
		ghttp.CookieOptions{
			SameSite: http.SameSiteLaxMode,
			Secure:   cfg.SessionSecureCookie,
			HttpOnly: true,
		},
	)
}

func clearPortalSSOStateCookie(r *ghttp.Request, svc *servicePortal.Service) {
	cfg := svc.Config()
	r.Cookie.SetCookie(
		portalSSOStateCookieName(svc),
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

func portalSSOStateCookieName(svc *servicePortal.Service) string {
	return svc.Config().SessionCookieName + portalSSOStateCookieSuffix
}

func sanitizePortalRedirectQuery(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/billing"
	}
	if strings.HasPrefix(trimmed, "/login") || strings.HasPrefix(trimmed, "/register") {
		return "/billing"
	}
	if parsed, err := url.Parse(trimmed); err != nil || parsed.Scheme != "" || parsed.Host != "" {
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

func ssoRedirectMessage(err error) string {
	if err == nil {
		return "SSO 登录失败"
	}
	return strings.TrimSpace(err.Error())
}
