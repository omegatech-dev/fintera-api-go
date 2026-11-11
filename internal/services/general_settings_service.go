package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"gorm.io/gorm"
)

// General settings validation errors.
var (
	ErrInvalidSupportWhatsApp = errors.New("el WhatsApp debe tener entre 8 y 20 dígitos, con código de país")
	ErrInvalidSupportEmail    = errors.New("el correo de soporte no es válido")
	ErrInvalidHoldHours       = errors.New("el bloqueo de reserva debe estar entre 1 y 720 horas")
)

var whatsappPattern = regexp.MustCompile(`^\+?[0-9]{8,20}$`)

// GeneralSettingsService manages the deployment-wide customer settings.
type GeneralSettingsService struct {
	repo  repository.GeneralSettingsRepository
	audit *AuditService
}

// NewGeneralSettingsService creates the settings service.
func NewGeneralSettingsService(repo repository.GeneralSettingsRepository, audit *AuditService) *GeneralSettingsService {
	return &GeneralSettingsService{repo: repo, audit: audit}
}

// Get returns the settings, applying defaults when the row is missing, and
// keeps the process-wide hold duration in sync.
func (s *GeneralSettingsService) Get(ctx context.Context) (*models.GeneralSettings, error) {
	settings, err := s.repo.Get(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		settings = models.DefaultGeneralSettings()
	} else if err != nil {
		return nil, err
	}
	applySettingsDefaults(settings)
	models.SetReservationHoldHours(settings.ReservationHoldHours)
	return settings, nil
}

// Update validates and persists the settings.
func (s *GeneralSettingsService) Update(ctx context.Context, actorID uint, payload models.GeneralSettings) (*models.GeneralSettings, error) {
	settings := &payload
	applySettingsDefaults(settings)
	if err := validateGeneralSettings(settings); err != nil {
		return nil, err
	}
	// Preserve the original creation timestamp on updates.
	if existing, err := s.repo.Get(ctx); err == nil {
		settings.CreatedAt = existing.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, err
	}
	models.SetReservationHoldHours(settings.ReservationHoldHours)
	logIgnoredError("audit settings update", s.audit.Log(ctx, actorID, "UPDATE", "GeneralSettings", settings.ID,
		fmt.Sprintf("Configuración general actualizada (bloqueo %d h)", settings.ReservationHoldHours), "", ""))
	return settings, nil
}

func applySettingsDefaults(settings *models.GeneralSettings) {
	if settings.ID == 0 {
		settings.ID = 1
	}
	settings.SupportWhatsApp = strings.TrimSpace(settings.SupportWhatsApp)
	settings.SupportEmail = strings.TrimSpace(settings.SupportEmail)
	if settings.ReservationHoldHours <= 0 {
		settings.ReservationHoldHours = models.DefaultReservationHoldHours
	}
}

func validateGeneralSettings(settings *models.GeneralSettings) error {
	if settings.SupportWhatsApp != "" && !whatsappPattern.MatchString(settings.SupportWhatsApp) {
		return ErrInvalidSupportWhatsApp
	}
	if settings.SupportEmail != "" {
		if _, err := mail.ParseAddress(settings.SupportEmail); err != nil {
			return ErrInvalidSupportEmail
		}
	}
	if settings.ReservationHoldHours < 1 || settings.ReservationHoldHours > 720 {
		return ErrInvalidHoldHours
	}
	return nil
}
