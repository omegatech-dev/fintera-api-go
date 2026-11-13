package services

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/storage"
	"gorm.io/gorm"
)

// Services holds all service instances
type Services struct {
	Auth           *AuthService
	User           *UserService
	Project        *ProjectService
	Lot            *LotService
	Contract       *ContractService
	Payment        *PaymentService
	Notification   *NotificationService
	Report         *ReportService
	Audit          *AuditService
	CreditScore    *CreditScoreService
	Email          *EmailService
	Analytics      *AnalyticsService
	Export         *ExportService
	Job            *JobService
	Branding       *BrandingService
	Settings       *GeneralSettingsService
	Public         *PublicCatalogService
	Media          *MediaService
	Instructions   *ProjectPaymentInstructionService
	Referral       *ReferralService
	Assignment     *AssignmentService
	Score          *ScoreService
	Seller         *SellerService
	Commission     *CommissionService
	HistoricalSale *HistoricalSaleService
}

// NewServices creates all service instances
func NewServices(repos *repository.Repositories, worker *jobs.Worker, storage *storage.LocalStorage, cfg *config.Config, db *gorm.DB) *Services {
	notificationSvc := NewNotificationService(repos.Notification, repos.User)
	emailSvc := NewEmailService(cfg)
	auditSvc := NewAuditService(db) // Create AuditService instance

	// Create ImageService
	imageSvc := NewImageService(cfg.StoragePath + "/uploads")

	analyticsSvc := NewAnalyticsService(repos.Analytics, repos.Project, notificationSvc, repos.User)
	jobSvc := NewJobService(worker)

	contractSvc := NewContractService(repos.Contract, repos.Lot, repos.User, repos.Payment, repos.Ledger, notificationSvc, emailSvc, auditSvc, worker)
	sellerSvc := NewSellerService(repos.Seller, repos.Assignment)
	referralSvc := NewReferralService(repos.Referral, repos.Seller, cfg)
	assignmentSvc := NewAssignmentService(repos.Assignment, repos.Contract, repos.User, notificationSvc, auditSvc)
	assignmentSvc.SetCommissionRepo(repos.Commission)
	scoreSvc := NewScoreService(repos.Seller, repos.Assignment, auditSvc)
	contractSvc.SetAssignmentServices(referralSvc, assignmentSvc)

	commissionSvc := NewCommissionService(repos.Commission, repos.User, auditSvc)
	referralSvc.SetCommissionService(commissionSvc)
	contractSvc.SetCommissionService(commissionSvc)
	historicalSvc := NewHistoricalSaleService(repos.HistoricalSale, commissionSvc, auditSvc)

	userSvc := NewUserService(repos.User, repos.Contract, worker, emailSvc, auditSvc, imageSvc)
	userSvc.SetSellerService(sellerSvc)
	userSvc.SetScoreService(scoreSvc)

	reportSvc := NewReportService(repos.Payment, repos.Contract, repos.User, repos.Instructions)
	reportSvc.SetHistoricalSaleRepository(repos.HistoricalSale)
	reportSvc.SetScoreService(scoreSvc)

	settingsSvc := NewGeneralSettingsService(repos.Settings, auditSvc)
	// Load the configured hold duration into the process-wide default.
	_, _ = settingsSvc.Get(context.Background())

	return &Services{
		Auth:           NewAuthService(repos.User, repos.RefreshToken, cfg),
		User:           userSvc,
		Project:        NewProjectService(repos.Project, repos.Lot, auditSvc),
		Lot:            NewLotService(repos.Lot, repos.Project, auditSvc),
		Contract:       contractSvc,
		Payment:        NewPaymentService(repos.Payment, repos.Contract, repos.Lot, repos.Ledger, contractSvc, notificationSvc, emailSvc, auditSvc, storage, worker),
		Notification:   notificationSvc,
		Report:         reportSvc,
		Audit:          auditSvc, // Assign AuditService
		CreditScore:    NewCreditScoreService(repos.User, repos.Contract, repos.Payment),
		Email:          emailSvc,
		Analytics:      analyticsSvc,
		Export:         NewExportService(analyticsSvc), // AnalyticsSvc passed to ExportSvc
		Job:            jobSvc,
		Branding:       NewBrandingService(repos.Branding, storage, auditSvc),
		Settings:       settingsSvc,
		Public:         NewPublicCatalogService(repos.Project, repos.Lot, repos.Instructions),
		Media:          NewMediaService(repos.Drawings, repos.ProjectPhoto, repos.LotPhoto, repos.Project, repos.Lot, storage, auditSvc),
		Instructions:   NewProjectPaymentInstructionService(repos.Instructions, repos.Project, auditSvc),
		Referral:       referralSvc,
		Assignment:     assignmentSvc,
		Score:          scoreSvc,
		Seller:         sellerSvc,
		Commission:     commissionSvc,
		HistoricalSale: historicalSvc,
	}
}
