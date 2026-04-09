package portal

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/httpx"
	"higress-portal-backend/internal/model"
	"higress-portal-backend/internal/service/portal"
)

type Controller struct {
	svc       *portal.Service
	webRoot   string
	indexFile string
	hasWeb    bool
}

func New(svc *portal.Service, webRoot string) *Controller {
	ctrl := &Controller{
		svc:     svc,
		webRoot: strings.TrimSpace(webRoot),
	}
	if ctrl.webRoot == "" {
		return ctrl
	}
	info, err := os.Stat(ctrl.webRoot)
	if err != nil || !info.IsDir() {
		return ctrl
	}
	index := filepath.Join(ctrl.webRoot, "index.html")
	if _, err = os.Stat(index); err != nil {
		return ctrl
	}
	ctrl.indexFile = index
	ctrl.hasWeb = true
	return ctrl
}

func (c *Controller) Health(r *ghttp.Request) {
	httpx.WriteJSON(r, http.StatusOK, g.Map{
		"status":  "ok",
		"service": "aigateway-portal-backend",
	})
}

func (c *Controller) Register(r *ghttp.Request) {
	var req model.RegisterRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}

	result, err := c.svc.Register(r.Context(), req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}

	if result.User.Status == consts.UserStatusActive {
		token, sessionErr := c.svc.CreateSession(r.Context(), result.User.ConsumerName)
		if sessionErr != nil {
			httpx.WriteError(r, sessionErr)
			return
		}
		setSessionCookie(r, token, c.svc)
	}

	httpx.WriteJSON(r, http.StatusCreated, g.Map{
		"user":          result.User,
		"defaultApiKey": result.DefaultAPIKey,
	})
}

func (c *Controller) Login(r *ghttp.Request) {
	var req model.LoginRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}

	user, err := c.svc.Login(r.Context(), req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}

	token, err := c.svc.CreateSession(r.Context(), user.ConsumerName)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	setSessionCookie(r, token, c.svc)
	httpx.WriteJSON(r, http.StatusOK, user)
}

func (c *Controller) Logout(r *ghttp.Request) {
	token := strings.TrimSpace(r.Cookie.Get(c.svc.Config().SessionCookieName).String())
	if token != "" {
		_ = c.svc.ClearSession(r.Context(), token)
	}
	clearSessionCookie(r, c.svc)
	httpx.WriteJSON(r, http.StatusOK, g.Map{"success": true})
}

func (c *Controller) Me(r *ghttp.Request) {
	httpx.WriteJSON(r, http.StatusOK, authUserFromRequest(r))
}

func (c *Controller) ChangePassword(r *ghttp.Request) {
	user := authUserFromRequest(r)
	var req model.ChangePasswordRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	if err := c.svc.ChangePassword(r.Context(), user.ConsumerName, req); err != nil {
		httpx.WriteError(r, err)
		return
	}
	clearSessionCookie(r, c.svc)
	httpx.WriteJSON(r, http.StatusOK, g.Map{"success": true})
}

func (c *Controller) BillingOverview(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	resp, err := c.svc.GetBillingOverview(r.Context(), targetConsumer)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) Consumptions(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	resp, err := c.svc.ListConsumptions(r.Context(), targetConsumer)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) Recharges(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	resp, err := c.svc.ListRecharges(r.Context(), targetConsumer)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) CreateRecharge(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	var req model.CreateRechargeRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.CreateRecharge(r.Context(), targetConsumer, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusCreated, resp)
}

func (c *Controller) ManagedAccounts(r *ghttp.Request) {
	user := authUserFromRequest(r)
	resp, err := c.svc.ListManagedAccounts(r.Context(), user.ConsumerName)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) ManagedDepartments(r *ghttp.Request) {
	user := authUserFromRequest(r)
	resp, err := c.svc.ListManagedDepartments(r.Context(), user.ConsumerName)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) UpdateManagedAccount(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer := r.Get("consumerName").String()
	var req model.UpdateManagedAccountRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.UpdateManagedAccount(r.Context(), user.ConsumerName, targetConsumer, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) AdjustManagedAccountBalance(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer := r.Get("consumerName").String()
	var req model.AdjustManagedAccountBalanceRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.AdjustManagedAccountBalance(r.Context(), user.ConsumerName, targetConsumer, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) ListModels(r *ghttp.Request) {
	resp, err := c.svc.ListModels(r.Context(), authUserFromRequest(r))
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) ModelDetail(r *ghttp.Request) {
	modelID := r.Get("id").String()
	resp, err := c.svc.GetModelDetail(r.Context(), modelID, authUserFromRequest(r))
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) ListAPIKeys(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	includeRaw := strings.EqualFold(strings.TrimSpace(r.Get("includeRaw").String()), "true")
	resp, err := c.svc.ListAPIKeys(r.Context(), targetConsumer, includeRaw)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) CreateAPIKey(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	var req model.CreateAPIKeyRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.CreateAPIKey(r.Context(), targetConsumer, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusCreated, resp)
}

func (c *Controller) UpdateAPIKeyStatus(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	keyID := r.Get("id").String()
	var req model.UpdateAPIKeyStatusRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.UpdateAPIKeyStatus(r.Context(), targetConsumer, keyID, strings.TrimSpace(req.Status))
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) UpdateAPIKey(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	keyID := r.Get("id").String()
	var req model.UpdateAPIKeyRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.UpdateAPIKey(r.Context(), targetConsumer, keyID, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) DeleteAPIKey(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	keyID := r.Get("id").String()
	if err := c.svc.DeleteAPIKey(r.Context(), targetConsumer, keyID); err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, g.Map{"id": keyID})
}

func (c *Controller) OpenStats(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	resp, err := c.svc.GetOpenStats(r.Context(), targetConsumer)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) CostDetails(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	resp, err := c.svc.ListCostDetails(r.Context(), targetConsumer)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) RequestDetails(r *ghttp.Request) {
	user := authUserFromRequest(r)
	targetConsumer, err := c.resolveAccessibleConsumer(r, user)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	resp, err := c.svc.ListRequestDetails(
		r.Context(),
		targetConsumer,
		strings.TrimSpace(r.Get("apiKeyId").String()),
		strings.TrimSpace(r.Get("modelId").String()),
		strings.TrimSpace(r.Get("routeName").String()),
		strings.TrimSpace(r.Get("requestStatus").String()),
		strings.TrimSpace(r.Get("usageStatus").String()),
		strings.TrimSpace(r.Get("startAt").String()),
		strings.TrimSpace(r.Get("endAt").String()),
		r.Get("pageNum").Int(),
		r.Get("pageSize").Int(),
	)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) DepartmentBillingSummary(r *ghttp.Request) {
	user := authUserFromRequest(r)
	includeChildren := true
	if raw := strings.TrimSpace(r.Get("includeChildren").String()); raw != "" {
		includeChildren = strings.EqualFold(raw, "true")
	}
	resp, err := c.svc.ListDepartmentBillingSummaries(
		r.Context(),
		user.ConsumerName,
		strings.TrimSpace(r.Get("departmentId").String()),
		includeChildren,
		strings.TrimSpace(r.Get("startDate").String()),
		strings.TrimSpace(r.Get("endDate").String()),
	)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) GetInvoiceProfile(r *ghttp.Request) {
	user := authUserFromRequest(r)
	resp, err := c.svc.GetInvoiceProfile(r.Context(), user.ConsumerName)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) UpdateInvoiceProfile(r *ghttp.Request) {
	user := authUserFromRequest(r)
	var req model.InvoiceProfile
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.UpdateInvoiceProfile(r.Context(), user.ConsumerName, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) InvoiceRecords(r *ghttp.Request) {
	user := authUserFromRequest(r)
	resp, err := c.svc.ListInvoiceRecords(r.Context(), user.ConsumerName)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusOK, resp)
}

func (c *Controller) CreateInvoice(r *ghttp.Request) {
	user := authUserFromRequest(r)
	var req model.CreateInvoiceRequest
	if err := r.Parse(&req); err != nil {
		httpx.WriteJSON(r, http.StatusBadRequest, g.Map{"message": "invalid request body"})
		return
	}
	resp, err := c.svc.CreateInvoice(r.Context(), user.ConsumerName, req)
	if err != nil {
		httpx.WriteError(r, err)
		return
	}
	httpx.WriteJSON(r, http.StatusCreated, resp)
}

func (c *Controller) Frontend(r *ghttp.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/") {
		httpx.WriteJSON(r, http.StatusNotFound, g.Map{"message": "not found"})
		return
	}
	if !c.hasWeb {
		httpx.WriteJSON(r, http.StatusNotFound, g.Map{"message": "not found"})
		return
	}

	cleaned := filepath.Clean("/" + path)
	if cleaned == "/" {
		r.Response.ServeFile(c.indexFile)
		return
	}

	rel := strings.TrimPrefix(cleaned, "/")
	candidate := filepath.Join(c.webRoot, rel)
	if fileInfo, err := os.Stat(candidate); err == nil && !fileInfo.IsDir() {
		r.Response.ServeFile(candidate)
		return
	}

	r.Response.ServeFile(c.indexFile)
}

func authUserFromRequest(r *ghttp.Request) model.AuthUser {
	var user model.AuthUser
	if v := r.GetCtxVar(consts.CtxUserKey); v != nil {
		_ = v.Struct(&user)
	}
	return user
}

func (c *Controller) resolveAccessibleConsumer(r *ghttp.Request, user model.AuthUser) (string, error) {
	return c.svc.ResolveAccessibleConsumer(r.Context(), user.ConsumerName, strings.TrimSpace(r.Get("consumerName").String()))
}
