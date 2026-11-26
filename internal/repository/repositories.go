package repository

import (
	"context"

	"gorm.io/gorm"
)

// Repositories holds all repository instances
type Repositories struct {
	db             *gorm.DB
	User           UserRepository
	Project        ProjectRepository
	Lot            LotRepository
	Contract       ContractRepository
	Payment        PaymentRepository
	Notification   NotificationRepository
	RefreshToken   RefreshTokenRepository
	Ledger         LedgerRepository
	Analytics      AnalyticsRepository
	Branding       BrandingRepository
	Settings       GeneralSettingsRepository
	Drawings       ProjectDrawingRepository
	ProjectPhoto   ProjectPhotoRepository
	LotPhoto       LotPhotoRepository
	Instructions   ProjectPaymentInstructionRepository
	Assignment     AssignmentRepository
	Referral       ReferralRepository
	Seller         SellerRepository
	Commission     CommissionRepository
	HistoricalSale HistoricalSaleRepository
	Backfill       BackfillRepository
	// Overdue serves the overdue payments export with the report-only query.
	Overdue OverdueReportRepository
	// IdentityDocument serves the DNI/RTN documents of a person and their
	// contract links.
	IdentityDocument IdentityDocumentRepository
}

// TransactionRunner executes a callback with transaction-scoped repositories.
type TransactionRunner func(context.Context, func(*Repositories) error) error

// NewRepositories creates all repository instances
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		db:               db,
		User:             NewUserRepository(db),
		Project:          NewProjectRepository(db),
		Lot:              NewLotRepository(db),
		Contract:         NewContractRepository(db),
		Payment:          NewPaymentRepository(db),
		Notification:     NewNotificationRepository(db),
		RefreshToken:     NewRefreshTokenRepository(db),
		Ledger:           NewLedgerRepository(db),
		Analytics:        NewAnalyticsRepository(db),
		Branding:         NewBrandingRepository(db),
		Settings:         NewGeneralSettingsRepository(db),
		Drawings:         NewProjectDrawingRepository(db),
		ProjectPhoto:     NewProjectPhotoRepository(db),
		LotPhoto:         NewLotPhotoRepository(db),
		Instructions:     NewProjectPaymentInstructionRepository(db),
		Assignment:       NewAssignmentRepository(db),
		Referral:         NewReferralRepository(db),
		Seller:           NewSellerRepository(db),
		Commission:       NewCommissionRepository(db),
		HistoricalSale:   NewHistoricalSaleRepository(db),
		Backfill:         NewBackfillRepository(db),
		Overdue:          NewOverdueReportRepository(db),
		IdentityDocument: NewIdentityDocumentRepository(db),
	}
}

// WithTx returns a repository bundle whose repositories share tx.
func (r *Repositories) WithTx(tx *gorm.DB) *Repositories {
	return NewRepositories(tx)
}

// DB returns the GORM handle backing this repository bundle. On a transaction
// scoped bundle this is the transaction handle.
func (r *Repositories) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

// RunInTx executes fn with a repository bundle bound to one database
// transaction. Any returned error rolls back all writes made through the
// transaction-scoped repositories.
func (r *Repositories) RunInTx(ctx context.Context, fn func(*Repositories) error) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(r.WithTx(tx))
	})
}
