package models

import (
	"sync/atomic"
	"time"
)

// DefaultReservationHoldHours is the fallback lot-hold duration.
const DefaultReservationHoldHours = 48

// GeneralSettings is the singleton row with deployment-wide customer settings.
type GeneralSettings struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	SupportWhatsApp      string    `gorm:"column:support_whatsapp;size:32" json:"support_whatsapp"`
	SupportEmail         string    `gorm:"column:support_email;size:160" json:"support_email"`
	ReservationHoldHours int       `gorm:"column:reservation_hold_hours;not null;default:48" json:"reservation_hold_hours"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

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
}

// ToResponse converts GeneralSettings to its response shape.
func (s *GeneralSettings) ToResponse() GeneralSettingsResponse {
	hours := s.ReservationHoldHours
	if hours <= 0 {
		hours = DefaultReservationHoldHours
	}
	return GeneralSettingsResponse{
		SupportWhatsApp:      s.SupportWhatsApp,
		SupportEmail:         s.SupportEmail,
		ReservationHoldHours: hours,
	}
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
