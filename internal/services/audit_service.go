package services

import (
	"context"
	"fmt"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// SystemAuditActor marks audit entries written without an authenticated user
// (jobs, unauthenticated flows). It is stored as a NULL user_id.
const SystemAuditActor uint = 0

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// WithTx returns an audit service whose writes share tx with the caller.
func (s *AuditService) WithTx(tx *gorm.DB) *AuditService {
	if s == nil {
		return nil
	}
	clone := *s
	clone.db = tx
	return &clone
}

// Log records an audit entry. A userID of SystemAuditActor is stored as NULL.
func (s *AuditService) Log(ctx context.Context, userID uint, action, entity string, entityID uint, details, ip, userAgent string) error {
	logEntry := &models.AuditLog{
		Action:    action,
		Entity:    entity,
		EntityID:  entityID,
		Details:   details,
		IPAddress: ip,
		UserAgent: userAgent,
	}
	if userID != SystemAuditActor {
		logEntry.UserID = &userID
	}
	if err := s.db.Create(logEntry).Error; err != nil {
		return fmt.Errorf("audit log %s %s #%d (actor %d): %w", action, entity, entityID, userID, err)
	}
	return nil
}

// List retrieves audit logs with filters (entity, search_term)
func (s *AuditService) List(ctx context.Context, limit, offset int, entity, entityID, searchTerm string) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	db := s.db.Model(&models.AuditLog{})
	if entity != "" {
		db = db.Where("audit_logs.entity = ?", entity)
	}
	if entityID != "" {
		db = db.Where("audit_logs.entity_id = ?", entityID)
	}
	if searchTerm != "" {
		pattern := "%" + searchTerm + "%"
		db = db.Joins("LEFT JOIN users ON users.id = audit_logs.user_id").
			Where("users.full_name ILIKE ? OR users.email ILIKE ? OR audit_logs.action ILIKE ? OR audit_logs.entity ILIKE ? OR audit_logs.details ILIKE ?",
				pattern, pattern, pattern, pattern, pattern)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	result := db.Preload("User").Order("audit_logs.created_at desc").Limit(limit).Offset(offset).Find(&logs)
	return logs, total, result.Error
}
