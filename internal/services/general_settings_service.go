package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"golang.org/x/net/html"
	"gorm.io/gorm"
)

// General settings validation errors.
var (
	ErrInvalidSupportWhatsApp = errors.New("el WhatsApp debe tener entre 8 y 20 dígitos, con código de país")
	ErrInvalidSupportEmail    = errors.New("el correo de soporte no es válido")
	ErrInvalidHoldHours       = errors.New("el bloqueo de reserva debe estar entre 1 y 720 horas")
	ErrInvalidWebsite         = errors.New("el sitio web debe ser una URL http(s) válida")
	ErrInvalidTermsDate       = errors.New("la fecha de vigencia debe tener formato AAAA-MM-DD")
	ErrLegalNameTooLong       = errors.New("la razón social no puede superar 160 caracteres")
	ErrLegalTaxIDTooLong      = errors.New("el RTN o identificación fiscal no puede superar 60 caracteres")
	ErrLegalAddressTooLong    = errors.New("la dirección legal no puede superar 500 caracteres")
	ErrWebsiteTooLong         = errors.New("el sitio web no puede superar 200 caracteres")
	ErrTermsContentTooLong    = errors.New("el contenido de los términos no puede superar 20000 caracteres")
)

// Maximum lengths accepted for the legal and terms fields.
const (
	maxLegalNameLength    = 160
	maxLegalTaxIDLength   = 60
	maxLegalAddressLength = 500
	maxWebsiteLength      = 200
	maxTermsContentLength = 20000
)

var whatsappPattern = regexp.MustCompile(`^\+?[0-9]{8,20}$`)

// termsHTMLPolicy is the allow-list used for the admin-authored terms. It
// mirrors the editor toolbar: paragraphs, emphasis, headings, lists, links and
// quotes. Images, inline styles, scripts and event handlers are discarded.
var termsHTMLPolicy = func() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowElements(
		"p", "br", "strong", "b", "em", "i", "u", "s", "a",
		"ul", "ol", "li", "h1", "h2", "h3", "blockquote",
	)
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowStandardURLs()
	policy.AllowURLSchemes("http", "https", "mailto")
	policy.RequireParseableURLs(true)
	policy.AllowRelativeURLs(false)
	policy.RequireNoFollowOnLinks(false)
	return policy
}()

// sanitizeTermsContent keeps only formatting allowed by the terms editor and
// drops markup that renders as an empty document.
func sanitizeTermsContent(value *string) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(termsHTMLPolicy.Sanitize(*value))
	if strings.TrimSpace(htmlVisibleText(clean)) == "" {
		return nil
	}
	return &clean
}

// htmlVisibleText returns the text a reader sees, ignoring markup. It is used
// to keep the 20000-character limit aligned with the UI character counter.
func htmlVisibleText(value string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(value))
	var text strings.Builder
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return text.String()
		case html.TextToken:
			text.Write(tokenizer.Text())
		}
	}
}

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
	settings.TermsContentES = sanitizeTermsContent(settings.TermsContentES)
	settings.TermsContentEN = sanitizeTermsContent(settings.TermsContentEN)
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
	if s.audit != nil {
		logIgnoredError("audit settings update", s.audit.Log(ctx, actorID, "UPDATE", "GeneralSettings", settings.ID,
			fmt.Sprintf("Configuración general actualizada (bloqueo %d h)", settings.ReservationHoldHours), "", ""))
	}
	return settings, nil
}

// ParseTermsEffectiveDate accepts an ISO date (YYYY-MM-DD) or an empty value.
func ParseTermsEffectiveDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(models.TermsDateLayout, value)
	if err != nil {
		return nil, ErrInvalidTermsDate
	}
	return &parsed, nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func applySettingsDefaults(settings *models.GeneralSettings) {
	if settings.ID == 0 {
		settings.ID = 1
	}
	settings.SupportWhatsApp = strings.TrimSpace(settings.SupportWhatsApp)
	settings.SupportEmail = strings.TrimSpace(settings.SupportEmail)
	settings.LegalName = strings.TrimSpace(settings.LegalName)
	settings.LegalTaxID = strings.TrimSpace(settings.LegalTaxID)
	settings.LegalAddress = trimOptionalString(settings.LegalAddress)
	settings.Website = strings.TrimSpace(settings.Website)
	settings.TermsContentES = trimOptionalString(settings.TermsContentES)
	settings.TermsContentEN = trimOptionalString(settings.TermsContentEN)
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
	if utf8.RuneCountInString(settings.LegalName) > maxLegalNameLength {
		return ErrLegalNameTooLong
	}
	if utf8.RuneCountInString(settings.LegalTaxID) > maxLegalTaxIDLength {
		return ErrLegalTaxIDTooLong
	}
	if settings.LegalAddress != nil && utf8.RuneCountInString(*settings.LegalAddress) > maxLegalAddressLength {
		return ErrLegalAddressTooLong
	}
	if settings.Website != "" {
		if utf8.RuneCountInString(settings.Website) > maxWebsiteLength {
			return ErrWebsiteTooLong
		}
		parsed, err := url.Parse(settings.Website)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return ErrInvalidWebsite
		}
	}
	if settings.TermsContentES != nil && utf8.RuneCountInString(htmlVisibleText(*settings.TermsContentES)) > maxTermsContentLength {
		return ErrTermsContentTooLong
	}
	if settings.TermsContentEN != nil && utf8.RuneCountInString(htmlVisibleText(*settings.TermsContentEN)) > maxTermsContentLength {
		return ErrTermsContentTooLong
	}
	return nil
}
