package models

import (
	"strings"
	"sync/atomic"
	"time"
)

// DefaultReservationHoldHours is the fallback lot-hold duration.
const DefaultReservationHoldHours = 48

// DefaultGeneralSettingsValues are the fallbacks for every policy field. They
// match the defaults the admin portal already sends, so a deployment that
// upgrades without touching the form keeps its behaviour.
const (
	DefaultReservationMinAmount              = 5000
	DefaultReservationDueDays                = 7
	DefaultDownPaymentDueDays                = 14
	DefaultDownPaymentDueDaysWithReservation = 21
	DefaultUpcomingReminderDays              = 1
	DefaultEmailNotificationsEnabled         = true
	DefaultOverdueRemindersEnabled           = true
	DefaultCurrency                          = "HNL"
	DefaultLocale                            = "es"
	DefaultTimezone                          = "America/Tegucigalpa"
)

// GeneralSettings is the singleton row with deployment-wide customer settings.
type GeneralSettings struct {
	ID                                uint       `gorm:"primaryKey" json:"id"`
	SupportWhatsApp                   string     `gorm:"column:support_whatsapp;size:32" json:"support_whatsapp"`
	SupportEmail                      string     `gorm:"column:support_email;size:160" json:"support_email"`
	ReservationHoldHours              int        `gorm:"column:reservation_hold_hours;not null;default:48" json:"reservation_hold_hours"`
	ReservationMinAmount              float64    `gorm:"column:reservation_min_amount;type:numeric(12,2);not null;default:5000" json:"reservation_min_amount"`
	ReservationDueDays                int        `gorm:"column:reservation_due_days;not null;default:7" json:"reservation_due_days"`
	DownPaymentDueDays                int        `gorm:"column:down_payment_due_days;not null;default:14" json:"down_payment_due_days"`
	DownPaymentDueDaysWithReservation int        `gorm:"column:down_payment_due_days_with_reservation;not null;default:21" json:"down_payment_due_days_with_reservation"`
	EmailNotificationsEnabled         bool       `gorm:"column:email_notifications_enabled;not null;default:true" json:"email_notifications_enabled"`
	OverdueRemindersEnabled           bool       `gorm:"column:overdue_reminders_enabled;not null;default:true" json:"overdue_reminders_enabled"`
	UpcomingReminderDays              int        `gorm:"column:upcoming_reminder_days;not null;default:1" json:"upcoming_reminder_days"`
	DefaultCurrency                   string     `gorm:"column:default_currency;size:3;not null;default:'HNL'" json:"default_currency"`
	DefaultLocale                     string     `gorm:"column:default_locale;size:8;not null;default:'es'" json:"default_locale"`
	Timezone                          string     `gorm:"column:timezone;size:64;not null;default:'America/Tegucigalpa'" json:"timezone"`
	LegalName                         string     `gorm:"column:legal_name;size:160;not null;default:''" json:"legal_name"`
	LegalTaxID                        string     `gorm:"column:legal_tax_id;size:60;not null;default:''" json:"legal_tax_id"`
	LegalAddress                      *string    `gorm:"column:legal_address" json:"legal_address"`
	Website                           string     `gorm:"column:website;size:200;not null;default:''" json:"website"`
	TermsEffectiveDate                *time.Time `gorm:"column:terms_effective_date;type:date" json:"terms_effective_date"`
	TermsContentES                    *string    `gorm:"column:terms_content_es" json:"terms_content_es"`
	TermsContentEN                    *string    `gorm:"column:terms_content_en" json:"terms_content_en"`
	CreatedAt                         time.Time  `json:"created_at"`
	UpdatedAt                         time.Time  `json:"updated_at"`
}

// GeneralSettingsPatch carries only the settings fields an update actually
// sent. A nil pointer means "not sent, keep the stored value"; a non-nil
// pointer carries the new value, including an empty string that clears it.
type GeneralSettingsPatch struct {
	SupportWhatsApp                   *string    `json:"support_whatsapp"`
	SupportEmail                      *string    `json:"support_email"`
	ReservationHoldHours              *int       `json:"reservation_hold_hours"`
	ReservationMinAmount              *float64   `json:"reservation_min_amount"`
	ReservationDueDays                *int       `json:"reservation_due_days"`
	DownPaymentDueDays                *int       `json:"down_payment_due_days"`
	DownPaymentDueDaysWithReservation *int       `json:"down_payment_due_days_with_reservation"`
	EmailNotificationsEnabled         *bool      `json:"email_notifications_enabled"`
	OverdueRemindersEnabled           *bool      `json:"overdue_reminders_enabled"`
	UpcomingReminderDays              *int       `json:"upcoming_reminder_days"`
	DefaultCurrency                   *string    `json:"default_currency"`
	DefaultLocale                     *string    `json:"default_locale"`
	Timezone                          *string    `json:"timezone"`
	LegalName                         *string    `json:"legal_name"`
	LegalTaxID                        *string    `json:"legal_tax_id"`
	LegalAddress                      *string    `json:"legal_address"`
	Website                           *string    `json:"website"`
	TermsEffectiveDate                *time.Time `json:"terms_effective_date"`
	TermsContentES                    *string    `json:"terms_content_es"`
	TermsContentEN                    *string    `json:"terms_content_en"`
}

// Apply overlays the patch on top of `settings` in place. Only the fields the
// caller sent are written, so the untouched fields keep their stored value.
func (p *GeneralSettingsPatch) Apply(settings *GeneralSettings) {
	if p.SupportWhatsApp != nil {
		settings.SupportWhatsApp = *p.SupportWhatsApp
	}
	if p.SupportEmail != nil {
		settings.SupportEmail = *p.SupportEmail
	}
	if p.ReservationHoldHours != nil {
		settings.ReservationHoldHours = *p.ReservationHoldHours
	}
	if p.ReservationMinAmount != nil {
		settings.ReservationMinAmount = *p.ReservationMinAmount
	}
	if p.ReservationDueDays != nil {
		settings.ReservationDueDays = *p.ReservationDueDays
	}
	if p.DownPaymentDueDays != nil {
		settings.DownPaymentDueDays = *p.DownPaymentDueDays
	}
	if p.DownPaymentDueDaysWithReservation != nil {
		settings.DownPaymentDueDaysWithReservation = *p.DownPaymentDueDaysWithReservation
	}
	if p.EmailNotificationsEnabled != nil {
		settings.EmailNotificationsEnabled = *p.EmailNotificationsEnabled
	}
	if p.OverdueRemindersEnabled != nil {
		settings.OverdueRemindersEnabled = *p.OverdueRemindersEnabled
	}
	if p.UpcomingReminderDays != nil {
		settings.UpcomingReminderDays = *p.UpcomingReminderDays
	}
	if p.DefaultCurrency != nil {
		settings.DefaultCurrency = *p.DefaultCurrency
	}
	if p.DefaultLocale != nil {
		settings.DefaultLocale = *p.DefaultLocale
	}
	if p.Timezone != nil {
		settings.Timezone = *p.Timezone
	}
	if p.LegalName != nil {
		settings.LegalName = *p.LegalName
	}
	if p.LegalTaxID != nil {
		settings.LegalTaxID = *p.LegalTaxID
	}
	if p.LegalAddress != nil {
		trimmed := strings.TrimSpace(*p.LegalAddress)
		if trimmed == "" {
			settings.LegalAddress = nil
		} else {
			settings.LegalAddress = &trimmed
		}
	}
	if p.Website != nil {
		settings.Website = *p.Website
	}
	if p.TermsEffectiveDate != nil {
		settings.TermsEffectiveDate = p.TermsEffectiveDate
	}
	if p.TermsContentES != nil {
		trimmed := strings.TrimSpace(*p.TermsContentES)
		if trimmed == "" {
			settings.TermsContentES = nil
		} else {
			settings.TermsContentES = &trimmed
		}
	}
	if p.TermsContentEN != nil {
		trimmed := strings.TrimSpace(*p.TermsContentEN)
		if trimmed == "" {
			settings.TermsContentEN = nil
		} else {
			settings.TermsContentEN = &trimmed
		}
	}
}

// TermsDateLayout is the ISO date format exposed by the settings API.
const TermsDateLayout = "2006-01-02"

// TableName specifies the table name for GeneralSettings.
func (GeneralSettings) TableName() string {
	return "general_settings"
}

// DefaultGeneralSettings returns the settings used when no row exists yet.
func DefaultGeneralSettings() *GeneralSettings {
	return &GeneralSettings{
		ID:                                1,
		ReservationHoldHours:              DefaultReservationHoldHours,
		ReservationMinAmount:              DefaultReservationMinAmount,
		ReservationDueDays:                DefaultReservationDueDays,
		DownPaymentDueDays:                DefaultDownPaymentDueDays,
		DownPaymentDueDaysWithReservation: DefaultDownPaymentDueDaysWithReservation,
		EmailNotificationsEnabled:         DefaultEmailNotificationsEnabled,
		OverdueRemindersEnabled:           DefaultOverdueRemindersEnabled,
		UpcomingReminderDays:              DefaultUpcomingReminderDays,
		DefaultCurrency:                   DefaultCurrency,
		DefaultLocale:                     DefaultLocale,
		Timezone:                          DefaultTimezone,
	}
}

// GeneralSettingsResponse is the public/admin JSON representation.
type GeneralSettingsResponse struct {
	SupportWhatsApp string `json:"support_whatsapp"`
	SupportEmail    string `json:"support_email"`
	// SupportsPartialUpdates is always true from the release that ships the
	// merge. A client keys its save strategy off this flag: when it is absent
	// the endpoint replaces the row and a partial write would erase the other
	// fields.
	SupportsPartialUpdates            bool    `json:"supports_partial_updates"`
	ReservationHoldHours              int     `json:"reservation_hold_hours"`
	ReservationMinAmount              float64 `json:"reservation_min_amount"`
	ReservationDueDays                int     `json:"reservation_due_days"`
	DownPaymentDueDays                int     `json:"down_payment_due_days"`
	DownPaymentDueDaysWithReservation int     `json:"down_payment_due_days_with_reservation"`
	EmailNotificationsEnabled         bool    `json:"email_notifications_enabled"`
	OverdueRemindersEnabled           bool    `json:"overdue_reminders_enabled"`
	UpcomingReminderDays              int     `json:"upcoming_reminder_days"`
	DefaultCurrency                   string  `json:"default_currency"`
	DefaultLocale                     string  `json:"default_locale"`
	Timezone                          string  `json:"timezone"`
	LegalName                         string  `json:"legal_name"`
	LegalTaxID                        string  `json:"legal_tax_id"`
	LegalAddress                      string  `json:"legal_address"`
	Website                           string  `json:"website"`
	TermsEffectiveDate                string  `json:"terms_effective_date"`
	TermsContentES                    string  `json:"terms_content_es"`
	TermsContentEN                    string  `json:"terms_content_en"`
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
		SupportWhatsApp:                   s.SupportWhatsApp,
		SupportEmail:                      s.SupportEmail,
		SupportsPartialUpdates:            true,
		ReservationHoldHours:              hours,
		ReservationMinAmount:              nonNegative(s.ReservationMinAmount, DefaultReservationMinAmount),
		ReservationDueDays:                positive(s.ReservationDueDays, DefaultReservationDueDays),
		DownPaymentDueDays:                positive(s.DownPaymentDueDays, DefaultDownPaymentDueDays),
		DownPaymentDueDaysWithReservation: positive(s.DownPaymentDueDaysWithReservation, DefaultDownPaymentDueDaysWithReservation),
		EmailNotificationsEnabled:         s.EmailNotificationsEnabled,
		OverdueRemindersEnabled:           s.OverdueRemindersEnabled,
		UpcomingReminderDays:              nonNegative(s.UpcomingReminderDays, DefaultUpcomingReminderDays),
		DefaultCurrency:                   firstNonEmpty(s.DefaultCurrency, DefaultCurrency),
		DefaultLocale:                     firstNonEmpty(s.DefaultLocale, DefaultLocale),
		Timezone:                          firstNonEmpty(s.Timezone, DefaultTimezone),
		LegalName:                         s.LegalName,
		LegalTaxID:                        s.LegalTaxID,
		LegalAddress:                      derefString(s.LegalAddress),
		Website:                           s.Website,
		TermsEffectiveDate:                termsDate,
		TermsContentES:                    derefString(s.TermsContentES),
		TermsContentEN:                    derefString(s.TermsContentEN),
	}
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func nonNegative[T int | float64](value, fallback T) T {
	if value < 0 {
		return fallback
	}
	return value
}

func positive(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// ToPolicy converts the settings row into the process-wide policy snapshot.
func (s *GeneralSettings) ToPolicy() SettingsPolicy {
	return SettingsPolicy{
		HoldHours:                         ReservationHoldHoursValue(s.ReservationHoldHours),
		MinAmount:                         nonNegative(s.ReservationMinAmount, DefaultReservationMinAmount),
		ReservationDueDays:                positive(s.ReservationDueDays, DefaultReservationDueDays),
		DownPaymentDueDays:                positive(s.DownPaymentDueDays, DefaultDownPaymentDueDays),
		DownPaymentDueDaysWithReservation: positive(s.DownPaymentDueDaysWithReservation, DefaultDownPaymentDueDaysWithReservation),
		EmailNotificationsEnabled:         s.EmailNotificationsEnabled,
		OverdueRemindersEnabled:           s.OverdueRemindersEnabled,
		UpcomingReminderDays:              nonNegative(s.UpcomingReminderDays, DefaultUpcomingReminderDays),
		DefaultCurrency:                   firstNonEmpty(s.DefaultCurrency, DefaultCurrency),
		DefaultLocale:                     firstNonEmpty(s.DefaultLocale, DefaultLocale),
		Timezone:                          firstNonEmpty(s.Timezone, DefaultTimezone),
	}
}

// ReservationHoldHoursValue resolves a stored hold duration against the
// default, so a row created before the setting existed still works.
func ReservationHoldHoursValue(stored int) int {
	if stored <= 0 {
		return DefaultReservationHoldHours
	}
	return stored
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// SettingsPolicy is the process-wide snapshot of the deployment-wide policy.
// Response mappers and services that do not take the settings service as a
// dependency read it. The singleton settings service keeps it in sync on read
// and update.
type SettingsPolicy struct {
	HoldHours                         int
	MinAmount                         float64
	ReservationDueDays                int
	DownPaymentDueDays                int
	DownPaymentDueDaysWithReservation int
	EmailNotificationsEnabled         bool
	OverdueRemindersEnabled           bool
	UpcomingReminderDays              int
	DefaultCurrency                   string
	DefaultLocale                     string
	Timezone                          string
}

var settingsPolicy atomic.Value // stores SettingsPolicy

func init() {
	settingsPolicy.Store(defaultSettingsPolicy())
}

func defaultSettingsPolicy() SettingsPolicy {
	return SettingsPolicy{
		HoldHours:                         DefaultReservationHoldHours,
		MinAmount:                         DefaultReservationMinAmount,
		ReservationDueDays:                DefaultReservationDueDays,
		DownPaymentDueDays:                DefaultDownPaymentDueDays,
		DownPaymentDueDaysWithReservation: DefaultDownPaymentDueDaysWithReservation,
		EmailNotificationsEnabled:         DefaultEmailNotificationsEnabled,
		OverdueRemindersEnabled:           DefaultOverdueRemindersEnabled,
		UpcomingReminderDays:              DefaultUpcomingReminderDays,
		DefaultCurrency:                   DefaultCurrency,
		DefaultLocale:                     DefaultLocale,
		Timezone:                          DefaultTimezone,
	}
}

// SettingsPolicySnapshot returns the process-wide policy snapshot.
func SettingsPolicySnapshot() SettingsPolicy {
	if value, ok := settingsPolicy.Load().(SettingsPolicy); ok {
		return value
	}
	return defaultSettingsPolicy()
}

// SetSettingsPolicy replaces the process-wide policy snapshot.
func SetSettingsPolicy(policy SettingsPolicy) {
	settingsPolicy.Store(policy)
}

// ReservationHoldHours returns the configured hold duration in hours.
func ReservationHoldHours() int {
	if hours := SettingsPolicySnapshot().HoldHours; hours > 0 {
		return hours
	}
	return DefaultReservationHoldHours
}

// ReservationHoldDuration returns the configured hold duration.
func ReservationHoldDuration() time.Duration {
	return time.Duration(ReservationHoldHours()) * time.Hour
}

// HoldExpiresAt returns when a lot hold created at `created` expires.
func HoldExpiresAt(created time.Time) time.Time {
	return created.Add(ReservationHoldDuration())
}
