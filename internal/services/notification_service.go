package services

import (
	"context"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

type NotificationService struct {
	repo     repository.NotificationRepository
	userRepo repository.UserRepository
}

func NewNotificationService(repo repository.NotificationRepository, userRepo repository.UserRepository) *NotificationService {
	return &NotificationService{repo: repo, userRepo: userRepo}
}

// WithRepositories returns a notification service backed by the given
// transaction-scoped repositories.
func (s *NotificationService) WithRepositories(repos *repository.Repositories) *NotificationService {
	if s == nil || repos == nil {
		return nil
	}
	clone := *s
	clone.repo = repos.Notification
	clone.userRepo = repos.User
	return &clone
}

func (s *NotificationService) FindByID(ctx context.Context, id uint) (*models.Notification, error) {
	return s.repo.FindByID(ctx, id)
}

// NotificationPage is one cursor page of a user's notifications.
type NotificationPage struct {
	Notifications []models.Notification
	Total         int64
	UnreadCount   int64
	NextCursor    *uint
	HasMore       bool
}

// FindByUserPage loads up to limit notifications newer-to-older starting after
// cursorID (0 starts at the newest) and reports whether older rows remain.
func (s *NotificationService) FindByUserPage(ctx context.Context, userID uint, status string, cursorID uint, limit int) (*NotificationPage, error) {
	rows, total, err := s.repo.FindByUserPage(ctx, userID, status, cursorID, limit+1)
	if err != nil {
		return nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	unread, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return nil, err
	}

	page := &NotificationPage{
		Notifications: rows,
		Total:         total,
		UnreadCount:   unread,
		HasMore:       hasMore,
	}
	if hasMore && len(rows) > 0 {
		cursor := rows[len(rows)-1].ID
		page.NextCursor = &cursor
	}

	return page, nil
}

func (s *NotificationService) Create(ctx context.Context, notification *models.Notification) error {
	return s.repo.Create(ctx, notification)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, id uint) error {
	notification, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	notification.MarkAsRead()
	return s.repo.Update(ctx, notification)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uint) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

func (s *NotificationService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *NotificationService) NotifyUser(ctx context.Context, userID uint, title, message, notifType string) error {
	notification := &models.Notification{
		UserID:           userID,
		Title:            title,
		Message:          message,
		NotificationType: &notifType,
	}
	return s.repo.Create(ctx, notification)
}

func (s *NotificationService) NotifyAdmins(ctx context.Context, title, message, notifType string) error {
	admins, err := s.userRepo.FindAdmins(ctx)
	if err != nil {
		return err
	}
	for _, admin := range admins {
		notification := &models.Notification{
			UserID:           admin.ID,
			Title:            title,
			Message:          message,
			NotificationType: &notifType,
		}
		logIgnoredError("create admin notification", s.repo.Create(ctx, notification))
	}
	return nil
}

// NotifyAdminsStrict persists one notification per admin and returns the first
// write error. Transactional assignment flows use this method so a failed
// notification insert rolls back the associated state changes.
func (s *NotificationService) NotifyAdminsStrict(ctx context.Context, title, message, notifType string) error {
	if s == nil {
		return nil
	}
	admins, err := s.userRepo.FindAdmins(ctx)
	if err != nil {
		return err
	}
	for _, admin := range admins {
		notification := &models.Notification{
			UserID:           admin.ID,
			Title:            title,
			Message:          message,
			NotificationType: &notifType,
		}
		if err := s.repo.Create(ctx, notification); err != nil {
			return err
		}
	}
	return nil
}
