package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// ReferralAttributionCookieName is the cookie that carries the last visited
// seller link.
const ReferralAttributionCookieName = "fintera_referral"

// ReferralAttributionTTL is the attribution window for a referral visit.
const ReferralAttributionTTL = 30 * 24 * time.Hour

const referralDefaultSecret = "fintera-referral-attribution"

// ReferralService resolves public referral links, records visits and signs the
// attribution cookie.
type ReferralService struct {
	repo          repository.ReferralRepository
	sellers       repository.SellerRepository
	commissionSvc *CommissionService
	secret        []byte
	portalURL     string
	secure        bool
}

// SetCommissionService wires the rate resolver for the referral metrics.
func (s *ReferralService) SetCommissionService(commission *CommissionService) {
	s.commissionSvc = commission
}

// NewReferralService creates the referral service.
func NewReferralService(repo repository.ReferralRepository, sellers repository.SellerRepository, cfg *config.Config) *ReferralService {
	secret := referralDefaultSecret
	portalURL := "https://portal.fintera.app"
	secure := false
	if cfg != nil {
		if strings.TrimSpace(cfg.JWTSecret) != "" {
			secret = cfg.JWTSecret
		}
		if strings.TrimSpace(cfg.PortalURL) != "" {
			portalURL = strings.TrimRight(cfg.PortalURL, "/")
		}
		secure = cfg.Environment == "production"
	}
	return &ReferralService{
		repo:      repo,
		sellers:   sellers,
		secret:    []byte(secret),
		portalURL: portalURL,
		secure:    secure,
	}
}

// ResolveVisit looks up an active seller and records the visit. It returns the
// seller and the signed attribution value that should be stored in the cookie.
func (s *ReferralService) ResolveVisit(ctx context.Context, guid, path, ip, userAgent string, now time.Time) (*models.User, string, error) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil, "", ErrReferralNotFound
	}
	seller, err := s.sellers.FindActiveByReferralGUID(ctx, guid)
	if err != nil || seller == nil {
		return nil, "", ErrReferralNotFound
	}

	hash := SessionHash(ip, userAgent)
	visit := &models.ReferralVisit{
		SellerID:     seller.ID,
		ReferralGUID: guid,
		VisitedAt:    now.UTC(),
		SessionHash:  &hash,
	}
	if strings.TrimSpace(path) != "" {
		p := strings.TrimSpace(path)
		if len(p) > 500 {
			p = p[:500]
		}
		visit.Path = &p
	}
	if err := s.repo.RecordVisit(ctx, visit); err != nil {
		return nil, "", err
	}
	return seller, s.SignAttribution(guid, now), nil
}

// ResolveAttribution returns the seller referenced by an explicit GUID or by a
// valid signed attribution cookie. Invalid or expired values are ignored.
func (s *ReferralService) ResolveAttribution(ctx context.Context, explicitGUID, cookieValue string, now time.Time) (*models.User, string, error) {
	guid := strings.TrimSpace(explicitGUID)
	if guid != "" {
		if seller := s.findActiveSeller(ctx, guid); seller != nil {
			return seller, guid, nil
		}
	}

	if cookieValue != "" {
		if cookieGUID, ok := s.ParseAttribution(cookieValue, now); ok {
			if seller := s.findActiveSeller(ctx, cookieGUID); seller != nil {
				return seller, cookieGUID, nil
			}
		}
	}
	return nil, "", nil
}

func (s *ReferralService) findActiveSeller(ctx context.Context, guid string) *models.User {
	seller, err := s.sellers.FindActiveByReferralGUID(ctx, guid)
	if err != nil {
		return nil
	}
	return seller
}

// SignAttribution builds the signed cookie value: guid|expiryUnix|signature.
func (s *ReferralService) SignAttribution(guid string, now time.Time) string {
	expiry := now.Add(ReferralAttributionTTL).Unix()
	payload := fmt.Sprintf("%s|%d", guid, expiry)
	return payload + "|" + s.signature(payload)
}

// ParseAttribution validates the signature and expiry of a cookie value.
func (s *ReferralService) ParseAttribution(value string, now time.Time) (string, bool) {
	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return "", false
	}
	guid, expiryRaw, signature := parts[0], parts[1], parts[2]
	if guid == "" {
		return "", false
	}
	expiry, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil || now.Unix() > expiry {
		return "", false
	}
	payload := guid + "|" + expiryRaw
	if !hmac.Equal([]byte(signature), []byte(s.signature(payload))) {
		return "", false
	}
	return guid, true
}

func (s *ReferralService) signature(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// AttributionCookie builds the signed cookie for a resolved visit.
func (s *ReferralService) AttributionCookie(guid string, now time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     ReferralAttributionCookieName,
		Value:    s.SignAttribution(guid, now),
		Path:     "/",
		MaxAge:   int(ReferralAttributionTTL.Seconds()),
		Expires:  now.Add(ReferralAttributionTTL),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.cookieSameSite(),
	}
}

// ClearedAttributionCookie expires the attribution after a reservation uses it.
func (s *ReferralService) ClearedAttributionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     ReferralAttributionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.cookieSameSite(),
	}
}

// cookieSameSite allows cross-site reservation posts from the portal in
// production (None requires Secure). Local development uses Lax.
func (s *ReferralService) cookieSameSite() http.SameSite {
	if s.secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

// PortalRedirectURL appends the referral GUID to the portal URL.
func (s *ReferralService) PortalRedirectURL(guid string) string {
	parsed, err := url.Parse(s.portalURL)
	if err != nil {
		return s.portalURL
	}
	query := parsed.Query()
	query.Set("ref", guid)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// Metrics returns the seller referral summary for the requested period.
func (s *ReferralService) Metrics(ctx context.Context, sellerID uint, days int) (*models.ReferralMetrics, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	since := time.Now().AddDate(0, 0, -days)

	visits, err := s.repo.CountVisitsSince(ctx, sellerID, since)
	if err != nil {
		return nil, err
	}
	reservations, err := s.repo.CountAttributedReservationsSince(ctx, sellerID, since)
	if err != nil {
		return nil, err
	}
	commission, err := s.repo.SumGeneratedCommissionSince(ctx, sellerID, since)
	if err != nil {
		return nil, err
	}

	conversion := float64(0)
	if visits > 0 {
		conversion = math.Round(float64(reservations)/float64(visits)*10000) / 100
	}
	metrics := &models.ReferralMetrics{
		PeriodDays:             days,
		Visits:                 visits,
		AttributedReservations: reservations,
		ConversionRate:         conversion,
		GeneratedCommission:    commission,
	}
	if s.sellers != nil {
		if seller, err := s.sellers.FindByID(ctx, sellerID); err == nil && seller != nil {
			metrics.SellerName = seller.FullName
			metrics.ReferralGUID = seller.ReferralGUID
			metrics.CommissionRate = seller.CommissionRateOverride
		}
	}
	if metrics.CommissionRate == nil && s.commissionSvc != nil {
		if rate, err := s.commissionSvc.EffectiveRate(ctx, sellerID); err == nil {
			metrics.CommissionRate = &rate
		}
	}
	return metrics, nil
}

// Reservations lists reservations attributed to the seller.
func (s *ReferralService) Reservations(ctx context.Context, sellerID uint, limit int) ([]models.Contract, error) {
	return s.repo.ListAttributedReservations(ctx, sellerID, limit)
}

// SessionHash returns a stable, non-reversible visitor fingerprint.
func SessionHash(ip, userAgent string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ip) + "|" + strings.TrimSpace(userAgent)))
	return hex.EncodeToString(sum[:])
}
