package api

import (
	"github.com/gin-gonic/gin"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api/middleware"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/auth"
	"github.com/mooncorn/gshub/api/internal/services/email"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"github.com/mooncorn/gshub/api/internal/services/portalloc"
	"github.com/mooncorn/gshub/api/internal/services/stripe"
)

type Handlers struct {
	Config        *config.Config
	AuthHandler   *AuthHandler
	ServerHandler *ServerHandler
}

func NewHandlers(db *database.DB, cfg *config.Config, k8sClient *k8s.Client, portAllocService *portalloc.Service) *Handlers {
	authService := auth.NewService(db, cfg)
	emailService := email.NewService(cfg)
	stripeService := stripe.NewService(db, cfg, k8sClient, portAllocService, cfg.K8sNamespace)

	return &Handlers{
		Config:        cfg,
		AuthHandler:   NewAuthHandler(authService, emailService),
		ServerHandler: NewServerHandler(db, k8sClient, cfg, stripeService),
	}
}

// RegisterRoutes registers all API routes
func (h *Handlers) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})

	// Auth routes (public)
	authRoutes := r.Group("/auth")
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
	protected := r.Group("")
	protected.Use(middleware.AuthMiddleware(h.Config.JWTSecret))
	{
		// User profile
		protected.GET("/me", h.AuthHandler.GetProfile)
		protected.PATCH("/me", h.AuthHandler.UpdateProfile)

		// Server management
		protected.GET("/servers", h.ServerHandler.ListServers)
		protected.GET("/servers/:id", h.ServerHandler.GetServer)
		protected.POST("/servers/:id/stop", h.ServerHandler.StopServer)
		protected.POST("/servers/:id/start", h.ServerHandler.StartServer)
		protected.POST("/servers/checkout", h.ServerHandler.CreateCheckoutSession)
	}

	// Stripe webhook (public, signature verified)
	r.POST("/webhooks/stripe", h.ServerHandler.HandleStripeWebhook)
}
