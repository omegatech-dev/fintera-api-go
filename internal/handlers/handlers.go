package handlers

import (
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/sjperalta/fintera-api/internal/storage"
)

// Handlers holds all handler instances
type Handlers struct {
	Health       *HealthHandler
	Auth         *AuthHandler
	User         *UserHandler
	Project      *ProjectHandler
	Lot          *LotHandler
	Contract     *ContractHandler
	Payment      *PaymentHandler
	Notification *NotificationHandler
	Report       *ReportHandler
	Audit        *AuditHandler
	Analytics    *AnalyticsHandler
	Job          *JobHandler
	Branding     *BrandingHandler
	Settings     *GeneralSettingsHandler
	Public       *PublicHandler
	Media        *MediaHandler
	Instructions *PaymentInstructionHandler
}

// NewHandlers creates all handler instances
func NewHandlers(svcs *services.Services, storage *storage.LocalStorage) *Handlers {
	return &Handlers{
		Health:       NewHealthHandler(),
		Auth:         NewAuthHandler(svcs.Auth),
		User:         NewUserHandler(svcs.User, svcs.Payment, svcs.Contract),
		Project:      NewProjectHandler(svcs.Project, svcs.Media),
		Lot:          NewLotHandler(svcs.Lot),
		Contract:     NewContractHandler(svcs.Contract, storage),
		Payment:      NewPaymentHandler(svcs.Payment, storage),
		Notification: NewNotificationHandler(svcs.Notification),
		Report:       NewReportHandler(svcs.Report),
		Audit:        NewAuditHandler(svcs.Audit), // Pass AuditService
		Analytics:    NewAnalyticsHandler(svcs.Analytics, svcs.Export),
		Job:          NewJobHandler(svcs.Job),
		Branding:     NewBrandingHandler(svcs.Branding),
		Settings:     NewGeneralSettingsHandler(svcs.Settings),
		Public:       NewPublicHandler(svcs.Public, svcs.Media),
		Media:        NewMediaHandler(svcs.Media),
		Instructions: NewPaymentInstructionHandler(svcs.Instructions),
	}
}
