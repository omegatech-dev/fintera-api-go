package repository

import (
	"gorm.io/gorm"
)

// Repositories holds all repository instances
type Repositories struct {
	User         UserRepository
	Project      ProjectRepository
	Lot          LotRepository
	Contract     ContractRepository
	Payment      PaymentRepository
	Notification NotificationRepository
	RefreshToken RefreshTokenRepository
	Ledger       LedgerRepository
	Analytics    AnalyticsRepository
	Branding     BrandingRepository
	Settings     GeneralSettingsRepository
	Drawings     ProjectDrawingRepository
	ProjectPhoto ProjectPhotoRepository
	LotPhoto     LotPhotoRepository
	Instructions ProjectPaymentInstructionRepository
}

// NewRepositories creates all repository instances
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		User:         NewUserRepository(db),
		Project:      NewProjectRepository(db),
		Lot:          NewLotRepository(db),
		Contract:     NewContractRepository(db),
		Payment:      NewPaymentRepository(db),
		Notification: NewNotificationRepository(db),
		RefreshToken: NewRefreshTokenRepository(db),
		Ledger:       NewLedgerRepository(db),
		Analytics:    NewAnalyticsRepository(db),
		Branding:     NewBrandingRepository(db),
		Settings:     NewGeneralSettingsRepository(db),
		Drawings:     NewProjectDrawingRepository(db),
		ProjectPhoto: NewProjectPhotoRepository(db),
		LotPhoto:     NewLotPhotoRepository(db),
		Instructions: NewProjectPaymentInstructionRepository(db),
	}
}
