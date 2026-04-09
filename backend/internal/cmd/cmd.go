package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"

	"higress-portal-backend/internal/config"
	portalController "higress-portal-backend/internal/controller/portal"
	"higress-portal-backend/internal/middleware"
	servicePortal "higress-portal-backend/internal/service/portal"
)

var Main = gcmd.Command{
	Name:  "main",
	Usage: "main",
	Brief: "start portal backend server",
	Func:  mainFunc,
}

func mainFunc(ctx context.Context, parser *gcmd.Parser) (err error) {
	cfg := config.Load()
	svc, err := servicePortal.New(cfg)
	if err != nil {
		return err
	}
	defer svc.Close(ctx)

	rootCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	svc.StartUsageSync(rootCtx)
	svc.StartKeyAuthSync(rootCtx)
	svc.StartBillingSync(rootCtx)

	ctrl := portalController.New(svc, cfg.WebRoot)
	authMw := middleware.NewAuth(svc)

	s := g.Server()
	s.SetAddr(cfg.ListenAddr)

	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareCORS)

		group.Group("/api", func(api *ghttp.RouterGroup) {
			api.GET("/health", ctrl.Health)

			api.Group("/auth", func(auth *ghttp.RouterGroup) {
				auth.POST("/register", ctrl.Register)
				auth.POST("/login", ctrl.Login)
				auth.POST("/logout", ctrl.Logout)
				auth.Group("/", func(authed *ghttp.RouterGroup) {
					authed.Middleware(authMw.Handler)
					authed.GET("/me", ctrl.Me)
					authed.POST("/change-password", ctrl.ChangePassword)
				})
			})

			api.Group("/", func(biz *ghttp.RouterGroup) {
				biz.Middleware(authMw.Handler)
				biz.GET("/accounts/managed", ctrl.ManagedAccounts)
				biz.GET("/departments/managed", ctrl.ManagedDepartments)
				biz.PATCH("/accounts/:consumerName/profile", ctrl.UpdateManagedAccount)
				biz.POST("/accounts/:consumerName/balance-adjustments", ctrl.AdjustManagedAccountBalance)

				biz.GET("/billing/overview", ctrl.BillingOverview)
				biz.GET("/billing/consumptions", ctrl.Consumptions)
				biz.GET("/billing/recharges", ctrl.Recharges)
				biz.POST("/billing/recharges", ctrl.CreateRecharge)

				biz.GET("/models", ctrl.ListModels)
				biz.GET("/models/:id", ctrl.ModelDetail)

				biz.GET("/open-platform/keys", ctrl.ListAPIKeys)
				biz.POST("/open-platform/keys", ctrl.CreateAPIKey)
				biz.PUT("/open-platform/keys/:id", ctrl.UpdateAPIKey)
				biz.PATCH("/open-platform/keys/:id/status", ctrl.UpdateAPIKeyStatus)
				biz.DELETE("/open-platform/keys/:id", ctrl.DeleteAPIKey)
				biz.GET("/open-platform/stats", ctrl.OpenStats)
				biz.GET("/open-platform/cost-details", ctrl.CostDetails)
				biz.GET("/open-platform/request-details", ctrl.RequestDetails)
				biz.GET("/billing/departments/summary", ctrl.DepartmentBillingSummary)

				biz.GET("/invoices/profile", ctrl.GetInvoiceProfile)
				biz.PUT("/invoices/profile", ctrl.UpdateInvoiceProfile)
				biz.GET("/invoices/records", ctrl.InvoiceRecords)
				biz.POST("/invoices/records", ctrl.CreateInvoice)
			})
		})

		group.ALL("/*", ctrl.Frontend)
	})

	s.Run()
	return nil
}
