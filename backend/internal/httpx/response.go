package httpx

import (
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"higress-portal-backend/internal/apperr"
)

func WriteJSON(r *ghttp.Request, status int, data any) {
	r.Response.WriteHeader(status)
	r.Response.WriteJson(data)
}

func WriteError(r *ghttp.Request, err error) {
	if httpErr, ok := apperr.As(err); ok {
		resp := g.Map{
			"message": httpErr.Message,
		}
		if httpErr.Detail != "" {
			resp["error"] = httpErr.Detail
		}
		WriteJSON(r, httpErr.Status, resp)
		return
	}
	WriteJSON(r, http.StatusInternalServerError, g.Map{"message": "internal server error"})
}
