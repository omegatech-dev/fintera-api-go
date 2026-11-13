package models

import (
	"sync/atomic"
	"time"
)

// DefaultReservationHoldHours is the fallback lot-hold duration.
const DefaultReservationHoldHours = 48

// GeneralSettings is the singleton row with deployment-wide customer settings.
type GeneralSettings struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	SupportWhatsApp      string     `gorm:"column:support_whatsapp;size:32" json:"support_whatsapp"`
	SupportEmail         string     `gorm:"column:support_email;size:160" json:"support_email"`
	ReservationHoldHours int        `gorm:"column:reservation_hold_hours;not null;default:48" json:"reservation_hold_hours"`
	LegalName            string     `gorm:"column:legal_name;size:160;not null;default:''" json:"legal_name"`
	LegalTaxID           string     `gorm:"column:legal_tax_id;size:60;not null;default:''" json:"legal_tax_id"`
	LegalAddress         *string    `gorm:"column:legal_address" json:"legal_address"`
	Website              string     `gorm:"column:website;size:200;not null;default:''" json:"website"`
	TermsEffectiveDate   *time.Time `gorm:"column:terms_effective_date;type:date" json:"terms_effective_date"`
	TermsContentES       *string    `gorm:"column:terms_content_es" json:"terms_content_es"`
	TermsContentEN       *string    `gorm:"column:terms_content_en" json:"terms_content_en"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// TermsDateLayout is the ISO date format exposed by the settings API.
const TermsDateLayout = "2006-01-02"

// TableName specifies the table name for GeneralSettings.
func (GeneralSettings) TableName() string {
	return "general_settings"
}

// DefaultGeneralSettings returns the settings used when no row exists yet.
func DefaultGeneralSettings() *GeneralSettings {
	return &GeneralSettings{ID: 1, ReservationHoldHours: DefaultReservationHoldHours}
}

// GeneralSettingsResponse is the public/admin JSON representation.
type GeneralSettingsResponse struct {
	SupportWhatsApp      string `json:"support_whatsapp"`
	SupportEmail         string `json:"support_email"`
	ReservationHoldHours int    `json:"reservation_hold_hours"`
	LegalName            string `json:"legal_name"`
	LegalTaxID           string `json:"legal_tax_id"`
	LegalAddress         string `json:"legal_address"`
	Website              string `json:"website"`
	TermsEffectiveDate   string `json:"terms_effective_date"`
	TermsContentES       string `json:"terms_content_es"`
	TermsContentEN       string `json:"terms_content_en"`
}

// ToResponse converts GeneralSettings to its response shape.
func (s *GeneralSettings) ToResponse() GeneralSettingsResponse {
	hours := s.ReservationHoldHours
	if hours <= 0 {
		hours = DefaultReservationHoldHours
	}
	termsDate := ""
	if s.TermsEffectiveDate != nil && !s.TermsEffectiveDate.IsZero() {
		termsDate = s.TermsEffectiveDate.Format(TermsDateLayout)
	}
	return GeneralSettingsResponse{
		SupportWhatsApp:      s.SupportWhatsApp,
		SupportEmail:         s.SupportEmail,
		ReservationHoldHours: hours,
		LegalName:            s.LegalName,
		LegalTaxID:           s.LegalTaxID,
		LegalAddress:         derefString(s.LegalAddress),
		Website:              s.Website,
		TermsEffectiveDate:   termsDate,
		TermsContentES:       derefString(s.TermsContentES),
		TermsContentEN:       derefString(s.TermsContentEN),
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// The hold duration is used by response mappers and services that do not take
// the settings service as a dependency. The singleton settings service keeps
// this value in sync on read and update.
var reservationHoldHours atomic.Int64

func init() {
	reservationHoldHours.Store(DefaultReservationHoldHours)
}

// ReservationHoldHours returns the configured hold duration in hours.
func ReservationHoldHours() int {
	value := reservationHoldHours.Load()
	if value <= 0 {
		return DefaultReservationHoldHours
	}
	return int(value)
}

// SetReservationHoldHours updates the process-wide hold duration.
func SetReservationHoldHours(hours int) {
	if hours > 0 {
		reservationHoldHours.Store(int64(hours))
	}
}

// ReservationHoldDuration returns the configured hold duration.
func ReservationHoldDuration() time.Duration {
	return time.Duration(ReservationHoldHours()) * time.Hour
}

// HoldExpiresAt returns when a lot hold created at `created` expires.
func HoldExpiresAt(created time.Time) time.Time {
	return created.Add(ReservationHoldDuration())
}
