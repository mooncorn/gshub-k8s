package api

import (
	"github.com/gin-gonic/gin"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api/middleware"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/auth"
	"github.com/mooncorn/gshub/api/internal/services/email"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
)

type Handlers struct {
	Config        *config.Config
	AuthHandler   *AuthHandler
	ServerHandler *ServerHandler
}

func NewHandlers(db *database.DB, cfg *config.Config, k8sClient *k8s.Client) *Handlers {
	authService := auth.NewService(db, cfg)
	emailService := email.NewService(cfg)

	return &Handlers{
		Config:        cfg,
		AuthHandler:   NewAuthHandler(authService, emailService),
		ServerHandler: NewServerHandler(db, k8sClient),
	}
}

// RegisterRoutes registers all API routes
func (h *Handlers) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})

	// Auth routes (public)
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/register", h.AuthHandler.Register)
		authRoutes.POST("/login", h.AuthHandler.Login)
		authRoutes.POST("/logout", h.AuthHandler.Logout)
		authRoutes.POST("/refresh", h.AuthHandler.RefreshToken)
		authRoutes.POST("/verify-email", h.AuthHandler.VerifyEmail)
		authRoutes.POST("/resend-verification", h.AuthHandler.ResendVerification)
		authRoutes.POST("/forgot-password", h.AuthHandler.ForgotPassword)
		authRoutes.POST("/reset-password", h.AuthHandler.ResetPassword)
	}

	// Protected routes
	protected := r.Group("/api/v1")
	protected.Use(middleware.AuthMiddleware(h.Config.JWTSecret))
	{
		// User profile
		protected.GET("/me", h.AuthHandler.GetProfile)
		protected.PATCH("/me", h.AuthHandler.UpdateProfile)
	}
}
