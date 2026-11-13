package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	_ "github.com/joho/godotenv/autoload"
	"github.com/rollbar/rollbar-go"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/sjperalta/fintera-api/docs"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/database"
	"github.com/sjperalta/fintera-api/internal/handlers"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
	"github.com/sjperalta/fintera-api/pkg/logger"

	"github.com/gin-gonic/gin"
)

// @title Fintera API
// @version 1.0
// @description REST API for Fintera Real Estate Management System
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /api/v1
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Setup(cfg.Environment)

	// Initialize Rollbar when token is configured
	if cfg.RollbarToken != "" {
		rollbar.SetToken(cfg.RollbarToken)
		rollbar.SetEnvironment(cfg.RollbarEnvironment)
		rollbar.SetCodeVersion(cfg.RollbarCodeVersion)
		rollbar.SetServerRoot(cfg.RollbarServerRoot)
		// Scrub credentials and session data before sending items. The SDK
		// default only covers password|secret|token.
		rollbar.SetScrubFields(regexp.MustCompile(`(?i)password|passwd|secret|token|authorization|cookie|api[_-]?key|session|jwt`))
		logger.Info("Rollbar initialized", "environment", cfg.RollbarEnvironment)
	} else {
		// Reporting is a no-op without a token, so background jobs and tests
		// never attempt network calls to Rollbar.
		rollbar.SetEnabled(false)
	}

	// Warn if Resend email is not configured (API loads .env, not .production.env)
	if cfg.ResendAPIKey == "" || cfg.FromEmail == "" {
		logger.Warn("Resend email disabled: RESEND_API_KEY or FROM_EMAIL not set. Set them in .env and ensure the From domain is verified in Resend dashboard.")
	}

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		flushRollbar()
		os.Exit(1)
	}
	logger.Info("Connected to database")

	// Initialize storage
	store, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		flushRollbar()
		os.Exit(1)
	}
	logger.Info("Initialized local storage")

	// Initialize repositories
	repos := repository.NewRepositories(db)

	// Initialize background worker
	worker := jobs.NewWorker(cfg.WorkerCount)
	logger.Info("Started background worker", "goroutines", cfg.WorkerCount)

	// Initialize services
	svcs := services.NewServices(repos, worker, store, cfg, db)

	// Schedule recurring jobs
	scheduleJobs(worker, svcs)

	// Initialize handlers
	h := handlers.NewHandlers(svcs, store)

	// Setup router
	router := setupRouter(h, cfg)

	// Dynamic Swagger Info replacement for Development
	if cfg.Environment == "development" && cfg.DefaultEmail != "" {
		docs.SwaggerInfo.SwaggerTemplate = strings.ReplaceAll(docs.SwaggerInfo.SwaggerTemplate, "DEFAULT_EMAIL_PLACEHOLDER", cfg.DefaultEmail)
		docs.SwaggerInfo.SwaggerTemplate = strings.ReplaceAll(docs.SwaggerInfo.SwaggerTemplate, "DEFAULT_PASSWORD_PLACEHOLDER", cfg.DefaultPassword)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			flushRollbar()
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	// Shutdown background worker
	worker.Shutdown()
	logger.Info("Background worker stopped")

	// Close Rollbar and flush events before exit
	flushRollbar()

	logger.Info("Server exited gracefully")
}

func setupRouter(h *handlers.Handlers, cfg *config.Config) *gin.Engine {
	router := gin.New()

	// Global middleware
	if cfg.RollbarToken != "" {
		router.Use(rollbarRecoveryMiddleware())
	}
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORS(cfg.AllowedOrigins))
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	// Serve uploaded files
	// Verify that StoragePath is correct; it's usually an absolute path or relative to CWD
	// We want to serve /uploads from {StoragePath}/uploads
	router.Static("/uploads", cfg.StoragePath+"/uploads")
	// Payment receipts are stored directly under {StoragePath}/receipts and
	// reported as /receipts/<document path> by the payment response DTO.
	router.Static("/receipts", cfg.StoragePath+"/receipts")

	// Public seller referral links: resolve, record the visit and redirect with
	// the signed 30-day attribution. 60 requests per minute per IP.
	router.GET("/r/:guid", middleware.RateLimit(60, time.Minute), h.Referral.Redirect)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Redirect root to swagger
		router.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
		})

		// Swagger documentation
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// Health check (public)
		v1.GET("/health", h.Health.Index)

		// Branding (public read; writes are admin-only below)
		v1.GET("/branding", h.Branding.Show)

		// General settings (public read; writes are admin-only below)
		v1.GET("/settings", h.Settings.Show)

		// Authentication (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", h.Auth.Login)
			auth.POST("/refresh", h.Auth.Refresh)
			auth.POST("/logout", h.Auth.Logout)
		}

		// Password recovery (public)
		v1.POST("/users/send_recovery_code", h.User.SendRecoveryCode)
		v1.POST("/users/verify_recovery_code", h.User.VerifyRecoveryCode)
		v1.POST("/users/update_password_with_code", h.User.UpdatePasswordWithCode)

		// Public customer portal catalog (no auth; 60 req/min per IP)
		public := v1.Group("/public")
		public.Use(middleware.RateLimit(60, time.Minute))
		{
			public.GET("/projects", h.Public.ListProjects)
			public.GET("/projects/:guid", h.Public.ShowProject)
			public.GET("/projects/:guid/lots", h.Public.ListLots)
			public.GET("/projects/:guid/lots/:lot_guid", h.Public.ShowLot)
			public.GET("/projects/:guid/payment-instructions", h.Public.PaymentInstructions)
			public.GET("/projects/:guid/drawings", h.Public.ListDrawings)
			public.GET("/projects/:guid/photos", h.Public.ListProjectPhotos)
			public.GET("/projects/:guid/lots/:lot_guid/photos", h.Public.ListLotPhotos)

			// Guest reservation creation is tighter: 5 per IP per hour.
			public.POST("/projects/:guid/lots/:lot_guid/contracts",
				middleware.OptionalAuth(cfg.JWTSecret),
				middleware.RateLimit(5, time.Hour),
				h.Contract.CreateGuest,
			)

			// Record a referral visit without redirecting (60 req/min via the
			// group rate limit).
			public.POST("/referrals/:guid/visit", h.Referral.Visit)

			// The wizard uploads the reservation voucher before the guest has
			// a session; only pending reservation payments are accepted.
			public.POST("/payments/:payment_id/upload_receipt",
				middleware.RateLimit(10, time.Minute),
				h.Payment.UploadPublicReservationReceipt,
			)

			// Resend the account setup email for a guest reservation.
			public.POST("/reservations/:guid/resend-access",
				middleware.RateLimit(5, time.Hour),
				h.Contract.ResendGuestAccess,
			)
		}

		// Protected routes (requires authentication)
		protected := v1.Group("")
		protected.Use(middleware.Auth(cfg.JWTSecret))
		{
			// Contract tracking by public GUID (owner/admin/seller checked in the handler).
			protected.GET("/contracts/:guid", h.Contract.ShowByGUID)
			// Customer-requested early payment (partial or full capital payment).
			protected.POST("/contracts/:guid/prepayments", h.Contract.RequestPrepayment)

			// Admin-only routes
			admin := protected.Group("")
			admin.Use(middleware.RequireAdmin())
			{
				// User management (admin only; PUT /users/:user_id is below for admin or owner)

				admin.DELETE("/users/:user_id", h.User.Delete)
				admin.PUT("/users/:user_id/toggle_status", h.User.ToggleStatus)
				admin.POST("/users/:user_id/restore", h.User.Restore)
				admin.PATCH("/users/:user_id/score", h.Score.Override)

				// Contract approval/rejection/cancellation/delete (admin only)
				admin.DELETE("/projects/:project_id/lots/:lot_id/contracts/:contract_id", h.Contract.Delete)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/approve", h.Contract.Approve)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/reject", h.Contract.Reject)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/cancel", h.Contract.Cancel)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/reopen", h.Contract.Reopen)
				admin.PATCH("/contracts/:guid/commission/pay", h.Contract.PayCommission)
				admin.POST("/contracts/:guid/reminder", h.Contract.SendReminder)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/capital_repayment", h.Contract.CapitalRepayment)

				// Payment approval/rejection/undo (admin only)
				admin.POST("/payments/:payment_id/approve", h.Payment.Approve)
				admin.POST("/payments/:payment_id/reject", h.Payment.Reject)
				admin.POST("/payments/:payment_id/undo", h.Payment.Undo)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/approve", h.Payment.ApproveByContract)
				admin.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/reject", h.Payment.RejectByContract)

				// Project management (admin only)
				admin.POST("/projects", h.Project.Create)
				admin.PUT("/projects/:project_id", h.Project.Update)
				admin.DELETE("/projects/:project_id", h.Project.Delete)
				admin.POST("/projects/:project_id/approve", h.Project.Approve)
				admin.POST("/projects/:project_id/publish", h.Project.Publish)
				admin.POST("/projects/:project_id/unpublish", h.Project.Unpublish)
				admin.POST("/projects/import", h.Project.Import)
				admin.GET("/projects/import/template", h.Project.ImportTemplate)

				// Lot management (admin only)
				admin.POST("/projects/:project_id/lots", h.Lot.Create)
				admin.PUT("/projects/:project_id/lots/:lot_id", h.Lot.Update)
				admin.DELETE("/projects/:project_id/lots/:lot_id", h.Lot.Delete)

				// Job status (admin only)
				admin.GET("/jobs/status", h.Job.Status)

				// Branding (admin only)
				admin.PUT("/branding", h.Branding.Update)
				admin.POST("/branding/logo", h.Branding.UploadLogo)
				admin.DELETE("/branding/logo", h.Branding.DeleteLogo)

				// General settings (admin only)
				admin.PUT("/settings", h.Settings.Update)

				// Project media (admin only)
				admin.GET("/projects/:project_id/drawings", h.Media.ListDrawings)
				admin.POST("/projects/:project_id/drawings", h.Media.UploadDrawings)
				admin.PATCH("/drawings/:drawing_id", h.Media.UpdateDrawing)
				admin.DELETE("/drawings/:drawing_id", h.Media.DeleteDrawing)
				admin.POST("/projects/:project_id/photos", h.Media.UploadProjectPhotos)
				admin.GET("/projects/:project_id/photos", h.Media.ListProjectPhotos)
				admin.PATCH("/project-photos/:photo_id", h.Media.UpdateProjectPhoto)
				admin.DELETE("/project-photos/:photo_id", h.Media.DeleteProjectPhoto)
				admin.POST("/projects/:project_id/lots/:lot_id/photos", h.Media.UploadLotPhotos)
				admin.GET("/projects/:project_id/lots/:lot_id/photos", h.Media.ListLotPhotos)
				admin.PATCH("/photos/:photo_id", h.Media.UpdateLotPhoto)
				admin.DELETE("/photos/:photo_id", h.Media.DeleteLotPhoto)

				// Project payment instructions (admin only)
				admin.GET("/projects/:project_id/payment-instructions", h.Instructions.Index)
				admin.POST("/projects/:project_id/payment-instructions", h.Instructions.Create)
				admin.PATCH("/projects/:project_id/payment-instructions/:instruction_id", h.Instructions.Update)
				admin.DELETE("/projects/:project_id/payment-instructions/:instruction_id", h.Instructions.Delete)

				// Commission rules and per-seller exceptions (admin only; the
				// read-only rules endpoint is also available to sellers below).
				admin.PUT("/commissions/rules", h.Commission.UpdateRules)
				admin.GET("/commissions/exceptions", h.Commission.ListExceptions)
				admin.POST("/commissions/exceptions", h.Commission.UpsertException)
				admin.DELETE("/commissions/exceptions/:id", h.Commission.DeleteException)

				// Historical-sale validation (admin only). Validation never
				// creates a reservation.
				admin.POST("/historical_sales/:id/validate", h.HistoricalSale.Validate)
				admin.POST("/historical_sales/:id/reject", h.HistoricalSale.Reject)
			}

			// User data access (Admin, Seller, or Owner)
			userData := protected.Group("/users/:user_id")
			userData.Use(middleware.RequireAdminSellerOrOwner())
			{
				userData.GET("", h.User.Show)
				userData.GET("/contracts", h.User.Contracts)
				userData.GET("/payments", h.User.Payments)
				userData.GET("/payment_history", h.User.PaymentHistory)
				userData.GET("/summary", h.User.Summary)
				userData.GET("/notifications", h.Notification.IndexByUser)
				// Customer document downloads (owner-scoped by RequireAdminSellerOrOwner
				// and a contract ownership check inside the handlers).
				userData.GET("/promise_contract_pdf", h.User.PromiseContractPDF)
				userData.GET("/customer_record_pdf", h.User.CustomerRecordPDF)
				userData.GET("/balance_pdf", h.User.BalancePDF)
			}

			// Seller + Admin routes (viewing and managing sales)
			sellerAdmin := protected.Group("")
			sellerAdmin.Use(middleware.RequireRole("admin", "seller"))
			{
				// User viewing (seller/admin can list all users) and creation
				sellerAdmin.GET("/users", h.User.Index)
				sellerAdmin.GET("/users/stats", h.User.Stats)
				sellerAdmin.POST("/users", h.User.Create)

				// Active sellers for reservation assignment.
				sellerAdmin.GET("/sellers", h.Seller.Index)

				// Seller assignment inbox.
				sellerAdmin.GET("/assignments/pending", h.Assignment.Pending)
				sellerAdmin.POST("/assignments/:id/accept", h.Assignment.Accept)
				sellerAdmin.POST("/assignments/:id/reject", h.Assignment.Reject)

				// Seller referral metrics.
				sellerAdmin.GET("/vendedor/referrals", h.Referral.Metrics)
				sellerAdmin.GET("/vendedor/referrals/reservations", h.Referral.Reservations)

				// Commission rules are readable by sellers (the historical-sale
				// form shows the default rate); writes are admin-only.
				sellerAdmin.GET("/commissions/rules", h.Commission.ShowRules)

				// Historical sales: sellers manage their own records, admins can
				// list every record. Validation is admin-only (above).
				sellerAdmin.GET("/historical_sales", h.HistoricalSale.Index)
				sellerAdmin.POST("/historical_sales", h.HistoricalSale.Create)
				sellerAdmin.PUT("/historical_sales/:id", h.HistoricalSale.Update)
				sellerAdmin.POST("/historical_sales/import", h.HistoricalSale.Import)
				sellerAdmin.GET("/historical_sales/import/template", h.HistoricalSale.ImportTemplate)

				// Contract viewing (seller can view all contracts)
				sellerAdmin.GET("/contracts", h.Contract.Index)
				sellerAdmin.GET("/contracts/stats", h.Contract.GetStats)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id", h.Contract.Show)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id/ledger", h.Contract.Ledger)

				// Payment viewing (seller can view all payments)
				sellerAdmin.GET("/payments", h.Payment.Index)
				sellerAdmin.GET("/payments/statistics", h.Payment.Statistics)
				sellerAdmin.GET("/payments/stats", h.Payment.Stats)
				sellerAdmin.GET("/payments/:payment_id", h.Payment.Show)

				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments", h.Payment.IndexByContract)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id", h.Payment.ShowByContract)

				// Project/Lot viewing (seller can view all)
				sellerAdmin.GET("/projects", h.Project.Index)
				sellerAdmin.GET("/projects/:project_id", h.Project.Show)
				sellerAdmin.GET("/projects/:project_id/lots", h.Lot.Index)
				sellerAdmin.GET("/projects/:project_id/lots/:lot_id", h.Lot.Show)

				// Analytics (seller can view analytics)
				analytics := sellerAdmin.Group("/analytics")
				{
					analytics.GET("/overview", h.Analytics.Overview)
					analytics.GET("/distribution", h.Analytics.Distribution)
					analytics.GET("/performance", h.Analytics.Performance)
					analytics.GET("/sellers", h.Analytics.Sellers)
					analytics.GET("/export", h.Analytics.Export)
				}

				// Reports (seller can generate reports)
				sellerAdmin.GET("/reports/commissions", h.Report.Commissions)
				sellerAdmin.GET("/reports/commissions_csv", h.Report.CommissionsCSV)
				sellerAdmin.GET("/reports/total_revenue_csv", h.Report.TotalRevenueCSV)
				sellerAdmin.GET("/reports/overdue_payments_csv", h.Report.OverduePaymentsCSV)
				sellerAdmin.GET("/reports/user_balance_pdf", h.Report.UserBalancePDF)
				sellerAdmin.GET("/reports/user_promise_contract_pdf", h.Report.UserPromiseContractPDF)
				sellerAdmin.GET("/reports/user_rescission_contract_pdf", h.Report.UserRescissionContractPDF)
				sellerAdmin.GET("/reports/user_information_pdf", h.Report.UserInformationPDF)
				sellerAdmin.GET("/reports/customer_record_pdf", h.Report.CustomerRecordPDF)
				sellerAdmin.GET("/dashboard/seller", h.Report.SellerDashboard)

				// Audits (seller can view audit logs)
				sellerAdmin.GET("/audits", h.Audit.Index)
			}

			// All authenticated users (personal data access)
			// Profile update: admin or profile owner only (sellers cannot update other users' profiles)
			protected.PUT("/users/:user_id", middleware.RequireAdminOrOwner(), h.User.Update)
			// User can change their own password
			protected.PATCH("/users/:user_id/change_password", h.User.ChangePassword)
			// Customer portal first login: password change for the authenticated user.
			protected.POST("/auth/change_password", h.User.ChangeOwnPassword)
			protected.POST("/users/:user_id/resend_confirmation", h.User.ResendConfirmation)
			protected.PATCH("/users/:user_id/update_locale", h.User.UpdateLocale)
			protected.POST("/users/:user_id/picture", h.User.UploadProfilePicture)

			// Contract creation (users can create their own contracts)
			protected.POST("/projects/:project_id/lots/:lot_id/contracts", h.Contract.Create)
			protected.PATCH("/projects/:project_id/lots/:lot_id/contracts/:contract_id", h.Contract.Update)

			// Payment receipt upload (users can upload their own receipts)
			protected.POST("/payments/:payment_id/upload_receipt", h.Payment.UploadReceipt)
			protected.POST("/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/upload_receipt", h.Payment.UploadReceiptByContract)
			protected.GET("/payments/:payment_id/download_receipt", h.Payment.DownloadReceipt)

			// Notifications (users can manage their own notifications)
			// Static route first so "mark_all_as_read" is not matched as :notification_id
			notifications := protected.Group("/notifications")
			{
				notifications.GET("", h.Notification.Index)
				notifications.POST("/mark_all_as_read", h.Notification.MarkAllAsRead)
				notifications.POST("/:notification_id/mark_as_read", h.Notification.MarkAsRead)
				notifications.GET("/:notification_id", h.Notification.Show)
				notifications.DELETE("/:notification_id", h.Notification.Delete)
			}
		}
	}

	return router
}

func scheduleJobs(worker *jobs.Worker, svcs *services.Services) {
	// Use ScheduleEveryImmediate so jobs run once on startup (e.g. after Railway deploy/restart),
	// then at the given interval. Plain ScheduleEvery would wait for the first interval before
	// running, so with restarts or long intervals (8h, 12h) jobs might never run.

	// Check overdue payments every 12 hours
	worker.ScheduleEveryImmediate(12*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Checking overdue payments...")
		// 1. Calculate and apply overdue interest
		if err := svcs.Payment.CalculateOverdueInterest(ctx); err != nil {
			logger.Error("Error calculating overdue interest", "error", err)
		}

		// 2. Send notifications
		return svcs.Payment.CheckOverduePayments(ctx)
	})

	// Update credit scores every 8 hours
	worker.ScheduleEveryImmediate(8*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Updating credit scores...")
		return svcs.CreditScore.UpdateAllScores(ctx)
	})

	// Refresh analytics cache every 15 minutes
	worker.ScheduleEveryImmediate(15*time.Minute, func(ctx context.Context) error {
		logger.Info("[Job] Refreshing analytics cache...")
		return svcs.Analytics.RefreshCache(ctx)
	})

	// Release unpaid reservations every 8 hours
	worker.ScheduleEveryImmediate(8*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Releasing unpaid reservations...")
		return svcs.Contract.ReleaseUnpaidReservations(ctx)
	})

	// Nudge customers whose 48h hold is about to expire without a voucher.
	worker.ScheduleEveryImmediate(8*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Sending reservation voucher reminders...")
		return svcs.Contract.SendReservationVoucherReminders(ctx)
	})

	// Daily payment reminder emails for active users with active contracts
	worker.ScheduleEveryImmediate(24*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Sending daily payment reminder emails...")
		if err := svcs.Payment.SendDailyPaymentReminderEmails(ctx); err != nil {
			return err
		}
		return svcs.Payment.SendDailyUpcomingPaymentReminderEmails(ctx)
	})

	// Expire overdue pending assignments and reassign the reservation every 5
	// minutes. Runs once on startup to catch downtime.
	worker.ScheduleEveryImmediate(5*time.Minute, func(ctx context.Context) error {
		logger.Info("[Job] Expiring overdue seller assignments...")
		return svcs.Assignment.ExpireOverdue(ctx)
	})

	// Nightly seller score recomputation from the 90-day assignment window.
	worker.ScheduleEveryImmediate(24*time.Hour, func(ctx context.Context) error {
		logger.Info("[Job] Recomputing seller scores...")
		return svcs.Score.RecomputeAll(ctx)
	})

	logger.Info("Scheduled recurring jobs")
}

// flushRollbar blocks until queued Rollbar items are sent. It is safe to call
// when reporting is disabled.
func flushRollbar() {
	rollbar.Wait()
}

// rollbarRecoveryMiddleware catches panics and reports them to Rollbar before
// re-panicking so gin.Recovery() writes the 500 response.
//
// The SDK only captures a stack trace when the reported value implements error,
// and treats a bare string as a message without one. Panic values are therefore
// normalized to errors so panic("boom") is still reported with useful context
// (plain string or non-error panics would otherwise lose the stack trace).
func rollbarRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("panic: %v", r)
			}

			extras := map[string]interface{}{
				"method":     c.Request.Method,
				"path":       c.Request.URL.Path,
				"route":      c.FullPath(),
				"ip":         c.ClientIP(),
				"user_agent": c.Request.UserAgent(),
			}
			if userID, exists := c.Get("userID"); exists {
				extras["user_id"] = userID
			}
			if userRole, exists := c.Get("userRole"); exists {
				extras["user_role"] = userRole
			}

			rollbar.ErrorWithExtras(rollbar.ERR, err, extras)

			// Re-panic so Gin's recovery handles the response.
			panic(r)
		}()
		c.Next()
	}
}
