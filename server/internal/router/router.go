package router

import (
	"net/http"
	"qdl/server/internal/config"
	"qdl/server/internal/handler"
	"qdl/server/internal/middleware"
	"qdl/server/internal/model"
	"qdl/server/internal/web"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg *config.Config, db *gorm.DB) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	authHandler := handler.NewAuthHandler(db, cfg)
	api.POST("/auth/login", authHandler.Login)

	authed := api.Group("")
	authed.Use(middleware.Auth(cfg.JWT.Secret))
	authed.Use(middleware.OperationLogger(db))
	authed.GET("/auth/me", authHandler.Me)
	authed.GET("/dashboard/overview", handler.NewDashboardHandler(db).Overview)

	domainAccountHandler := handler.NewDomainAccountHandler(db)
	registerResource(authed, "/domain-accounts", domainAccountHandler)
	authed.POST("/domain-accounts/:id/test", domainAccountHandler.Test)
	sslAccountHandler := handler.NewSSLAccountHandler(db)
	registerResource(authed, "/ssl-accounts", sslAccountHandler)
	authed.GET("/ssl-accounts/:id/certificates", sslAccountHandler.Certificates)
	authed.POST("/ssl-accounts/:id/certificates/import", sslAccountHandler.ImportCertificates)
	domainHandler := handler.NewDomainHandler(db)
	registerResource(authed, "/domains", domainHandler)
	authed.GET("/domain-accounts/:id/provider-domains", domainHandler.ProviderDomains)
	authed.POST("/domains/:id/refresh-expires", domainHandler.RefreshExpires)
	authed.GET("/domains/:id/records", domainHandler.Records)
	authed.GET("/domains/:id/record-lines", domainHandler.RecordLines)
	authed.POST("/domains/:id/records", domainHandler.CreateRecord)
	authed.PUT("/domains/:id/records/:recordId", domainHandler.UpdateRecord)
	authed.DELETE("/domains/:id/records/:recordId", domainHandler.DeleteRecord)
	certificateHandler := handler.NewCertificateHandler(db)
	registerResource(authed, "/certificates", certificateHandler)
	authed.POST("/certificates/:id/submit", certificateHandler.Submit)
	authed.POST("/certificates/:id/revoke", certificateHandler.Revoke)
	authed.GET("/certificates/:id/detail", certificateHandler.Detail)
	registerResource(authed, "/tasks", handler.NewResourceHandler[model.AutoTask](db))
	registerResource(authed, "/users", handler.NewUserHandler(db))
	registerResource(authed, "/logs", handler.NewResourceHandler[model.OperationLog](db))
	registerResource(authed, "/login-logs", handler.NewResourceHandler[model.LoginLog](db))
	registerResource(authed, "/certificate-logs", handler.NewResourceHandler[model.CertificateLog](db))
	registerResource(authed, "/system-settings", handler.NewResourceHandler[model.SystemSetting](db))
	registerResource(authed, "/notification-settings", handler.NewResourceHandler[model.NotificationSetting](db))

	web.Register(r)

	return r
}

type resource interface {
	List(*gin.Context)
	Get(*gin.Context)
	Create(*gin.Context)
	Update(*gin.Context)
	Delete(*gin.Context)
}

func registerResource(group *gin.RouterGroup, path string, h resource) {
	g := group.Group(path)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.POST("", h.Create)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}
