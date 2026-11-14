package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	Email               string     `gorm:"uniqueIndex;not null" json:"email"`
	EncryptedPassword   string     `gorm:"column:encrypted_password;not null" json:"-"`
	ResetPasswordToken  *string    `json:"-"`
	ResetPasswordSentAt *time.Time `json:"-"`
	RememberCreatedAt   *time.Time `json:"-"`
	ConfirmationToken   *string    `json:"-"`
	ConfirmedAt         *time.Time `json:"confirmed_at"`
	ConfirmationSentAt  *time.Time `json:"-"`
	UnconfirmedEmail    *string    `json:"-"`
	Role                string     `gorm:"default:user" json:"role"`
	FullName            string     `json:"full_name"`
	Phone               string     `json:"phone"`
	Status              string     `gorm:"default:active" json:"status"`
	Identity            string     `gorm:"uniqueIndex" json:"identity"`
	RTN                 string     `gorm:"column:rtn;uniqueIndex" json:"rtn"`
	DiscardedAt         *time.Time `gorm:"index" json:"-"`
	RecoveryCode        *string    `json:"-"`
	RecoveryCodeSentAt  *time.Time `json:"-"`
	Address             *string    `json:"address"`
	CreatedBy           *uint      `json:"created_by"`
	Note                *string    `json:"note"`
	CreditScore         int        `gorm:"default:0" json:"credit_score"`
	Locale              string     `gorm:"default:es" json:"locale"`
	MustChangePassword  bool       `gorm:"column:must_change_password;default:false" json:"must_change_password"`
	ProfilePicture      *string    `json:"profile_picture"`
	ProfilePictureThumb *string    `gorm:"column:profile_picture_thumb" json:"profile_picture_thumb"`
	// Seller profile
	ReferralGUID           *string    `gorm:"column:referral_guid;type:uuid;index" json:"referral_guid"`
	CommissionRateOverride *float64   `gorm:"column:commission_rate_override;type:decimal(5,2)" json:"commission_rate_override"`
	Score                  int        `gorm:"default:100" json:"score"`
	ScoreLevel             string     `gorm:"column:score_level;size:20;default:high" json:"score_level"`
	ScoreOverride          *int       `gorm:"column:score_override" json:"score_override"`
	ScoreOverrideReason    *string    `gorm:"column:score_override_reason;type:text" json:"score_override_reason"`
	ScoreOverrideBy        *uint      `gorm:"column:score_override_by" json:"score_override_by"`
	ScoreOverrideAt        *time.Time `gorm:"column:score_override_at" json:"score_override_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`

	// Associations
	Creator       *User          `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Contracts     []Contract     `gorm:"foreignKey:ApplicantUserID" json:"contracts,omitempty"`
	Notifications []Notification `gorm:"foreignKey:UserID" json:"notifications,omitempty"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook for setting defaults
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Role == "" {
		u.Role = RoleUser
	}
	if u.Status == "" {
		u.Status = StatusActive
	}
	if u.Locale == "" {
		u.Locale = LocaleES
	}
	if u.Role == RoleSeller {
		if u.Score == 0 && u.ScoreOverride == nil {
			u.Score = 100
		}
		if u.ScoreLevel == "" {
			u.ScoreLevel = ScoreLevelFor(u.Score)
		}
	}
	return nil
}

// IsAdmin returns true if user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsSeller returns true if user has seller role
func (u *User) IsSeller() bool {
	return u.Role == RoleSeller
}

// IsActive returns true if user status is active
func (u *User) IsActive() bool {
	return u.Status == StatusActive && u.DiscardedAt == nil
}

// IsConfirmed returns true if user email is confirmed
func (u *User) IsConfirmed() bool {
	return u.ConfirmedAt != nil
}

// IsDiscarded returns true if user is soft-deleted
func (u *User) IsDiscarded() bool {
	return u.DiscardedAt != nil
}

// Role constants
const (
	RoleAdmin  = "admin"
	RoleSeller = "seller"
	RoleUser   = "user"
)

// Status constants
const (
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"
)

// Locale constants
const (
	LocaleES = "es"
	LocaleEN = "en"
)

// Score levels. A seller with 85 or more is high, 60..84 medium and below 60
// low. Only high and medium sellers receive automatic assignments.
const (
	ScoreLevelHigh   = "high"
	ScoreLevelMedium = "medium"
	ScoreLevelLow    = "low"

	SellerScoreWindowDays = 90
)

// ScoreLevelFor maps a numeric acceptance score to its level.
func ScoreLevelFor(score int) string {
	switch {
	case score >= 85:
		return ScoreLevelHigh
	case score >= 60:
		return ScoreLevelMedium
	default:
		return ScoreLevelLow
	}
}

// ComputeAcceptanceScore returns the score and level for accepted/expired
// assignment outcomes. No history scores 100 (high) instead of an undefined
// value.
func ComputeAcceptanceScore(accepted, expired int64) (int, string) {
	total := accepted + expired
	if total <= 0 {
		return 100, ScoreLevelHigh
	}
	score := int(float64(accepted)/float64(total)*100 + 0.5)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, ScoreLevelFor(score)
}

// IsAssignable reports whether the seller may receive automatic assignments.
func (u *User) IsAssignable() bool {
	return u.IsSeller() && u.IsActive() &&
		(u.ScoreLevel == ScoreLevelHigh || u.ScoreLevel == ScoreLevelMedium)
}

// ScoreMetrics is the accepted/expired breakdown behind a seller score. The UI
// renders it as "12 de 13 aceptadas · 90 días".
type ScoreMetrics struct {
	AcceptedCount int64 `json:"accepted_count"`
	ExpiredCount  int64 `json:"expired_count"`
	WindowDays    int   `json:"window_days"`
}

// Total returns the number of offers in the score window.
func (m ScoreMetrics) Total() int64 {
	return m.AcceptedCount + m.ExpiredCount
}

// EmptyScoreMetrics returns zeroed metrics with the standard window.
func EmptyScoreMetrics() ScoreMetrics {
	return ScoreMetrics{WindowDays: SellerScoreWindowDays}
}

// SellerSummary is the lightweight seller payload used by seller pickers.
type SellerSummary struct {
	ID         uint    `json:"id"`
	FullName   string  `json:"full_name"`
	Address    *string `json:"address"`
	SalesCount int64   `json:"sales_count"`
	Score      int     `json:"score"`
	ScoreLevel string  `json:"score_level"`
}

// UserResponse is the JSON response format for users
type UserResponse struct {
	ID                     uint       `json:"id"`
	Email                  string     `json:"email"`
	FullName               string     `json:"full_name"`
	Phone                  string     `json:"phone"`
	Role                   string     `json:"role"`
	Status                 string     `json:"status"`
	Identity               string     `json:"identity"`
	RTN                    string     `json:"rtn"`
	Address                *string    `json:"address"`
	CreditScore            int        `json:"credit_score"`
	Locale                 string     `json:"locale"`
	MustChangePassword     bool       `json:"must_change_password"`
	ProfilePicture         *string    `json:"profile_picture"`
	ProfilePictureThumb    *string    `json:"profile_picture_thumb"`
	ReferralGUID           *string    `json:"referral_guid"`
	CommissionRateOverride *float64   `json:"commission_rate_override"`
	Score                  int        `json:"score"`
	ScoreLevel             string     `json:"score_level"`
	ScoreOverride          *int       `json:"score_override"`
	ScoreOverrideReason    *string    `json:"score_override_reason"`
	ScoreOverrideBy        *uint      `json:"score_override_by"`
	ScoreOverrideAt        *time.Time `json:"score_override_at"`
	ConfirmedAt            *time.Time `json:"confirmed_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() UserResponse {
	level := u.ScoreLevel
	if level == "" {
		level = ScoreLevelFor(u.Score)
	}
	return UserResponse{
		ID:                     u.ID,
		Email:                  u.Email,
		FullName:               u.FullName,
		Phone:                  u.Phone,
		Role:                   u.Role,
		Status:                 u.Status,
		Identity:               u.Identity,
		RTN:                    u.RTN,
		Address:                u.Address,
		CreditScore:            u.CreditScore,
		Locale:                 u.Locale,
		MustChangePassword:     u.MustChangePassword,
		ProfilePicture:         u.ProfilePicture,
		ProfilePictureThumb:    u.ProfilePictureThumb,
		ReferralGUID:           u.ReferralGUID,
		CommissionRateOverride: u.CommissionRateOverride,
		Score:                  u.Score,
		ScoreLevel:             level,
		ScoreOverride:          u.ScoreOverride,
		ScoreOverrideReason:    u.ScoreOverrideReason,
		ScoreOverrideBy:        u.ScoreOverrideBy,
		ScoreOverrideAt:        u.ScoreOverrideAt,
		ConfirmedAt:            u.ConfirmedAt,
		CreatedAt:              u.CreatedAt,
		UpdatedAt:              u.UpdatedAt,
	}
}
