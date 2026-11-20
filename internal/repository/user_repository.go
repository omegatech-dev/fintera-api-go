package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByIdentity(ctx context.Context, identity string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	// CreateHistoricalClient inserts a migrated customer without an RTN so the
	// unique RTN index is not hit by multiple clients without a tax id.
	CreateHistoricalClient(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	// UpdateCreditScore writes only credit_score and updated_at, leaving all
	// other user columns untouched.
	UpdateCreditScore(ctx context.Context, userID uint, score int) error
	SetRecoveryCode(ctx context.Context, userID uint, code string, sentAt time.Time) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error
	Restore(ctx context.Context, id uint) error
	List(ctx context.Context, query *ListQuery) ([]models.User, int64, error)
	FindAdmins(ctx context.Context) ([]models.User, error)
	FindAll(ctx context.Context) ([]models.User, error)
	CountByRole(ctx context.Context) (map[string]int64, error)
	CountHistoricalClients(ctx context.Context) (int64, error)
	CountActiveClientsByCreator(ctx context.Context, creatorIDs []uint) (map[uint]int64, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) CountByRole(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Role  string
		Count int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("role, count(*) as count").
		Where("discarded_at IS NULL").
		Group("role").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, item := range rows {
		counts[item.Role] = item.Count
	}
	return counts, nil
}

func (r *userRepository) CountHistoricalClients(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("discarded_at IS NULL AND historical_client = ?", true).
		Count(&total).Error
	return total, err
}

// CountActiveClientsByCreator returns, for each creator, how many active users
// with the client role they created. Creators without clients are omitted (the
// caller defaults them to zero).
func (r *userRepository) CountActiveClientsByCreator(ctx context.Context, creatorIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(creatorIDs))
	if len(creatorIDs) == 0 {
		return counts, nil
	}

	type row struct {
		CreatedBy uint
		Count     int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("created_by, count(*) as count").
		Where("discarded_at IS NULL").
		Where("role = ?", models.RoleUser).
		Where("status = ?", models.StatusActive).
		Where("created_by IN ?", creatorIDs).
		Group("created_by").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, item := range rows {
		if item.CreatedBy != 0 {
			counts[item.CreatedBy] = item.Count
		}
	}
	return counts, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("discarded_at IS NULL").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.User, error) {
	if len(ids) == 0 {
		return []models.User{}, nil
	}
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("id IN ? AND discarded_at IS NULL", ids).
		Find(&users).Error
	return users, err
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("LOWER(email) = LOWER(?) AND discarded_at IS NULL", email).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByIdentity(ctx context.Context, identity string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("identity = ? AND discarded_at IS NULL", identity).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// emptyUniqueColumns lists the GORM field names of the unique columns that are
// empty. They are omitted from INSERT/UPDATE and stored as NULL (migration
// 000046 replaced users_rtn_key/users_identity_key with partial unique indexes
// that exclude empty values), so users without an RTN or identity never
// collide with each other on save.
func emptyUniqueColumns(user *models.User) []string {
	columns := make([]string, 0, 2)
	if user.RTN == "" {
		columns = append(columns, "RTN")
	}
	if user.Identity == "" {
		columns = append(columns, "Identity")
	}
	return columns
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Omit(emptyUniqueColumns(user)...).Create(user).Error; err != nil {
		if isDuplicateKeyError(err, "users_identity_key", "idx_users_identity_unique") {
			return errors.New("ya existe un usuario con este documento de identidad")
		}
		if isDuplicateKeyError(err, "users_email_key") {
			return errors.New("ya existe un usuario con este correo electrónico")
		}
		if isDuplicateKeyError(err, "users_rtn_key", "idx_users_rtn_unique") {
			return errors.New("ya existe un usuario con este RTN")
		}
		return err
	}
	return nil
}

func (r *userRepository) CreateHistoricalClient(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Omit(emptyUniqueColumns(user)...).Create(user).Error; err != nil {
		if isDuplicateKeyError(err, "users_identity_key", "idx_users_identity_unique") {
			return errors.New("ya existe un usuario con este documento de identidad")
		}
		if isDuplicateKeyError(err, "users_email_key") {
			return errors.New("ya existe un usuario con este correo electrónico")
		}
		return err
	}
	return nil
}

func isDuplicateKeyError(err error, constraintNames ...string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && slices.Contains(constraintNames, pgErr.ConstraintName)
	}
	return false
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	empty := emptyUniqueColumns(user)
	if err := r.db.WithContext(ctx).Omit(empty...).Save(user).Error; err != nil {
		return err
	}
	// Save skips the omitted columns, so a value cleared in this edit must be
	// written as NULL explicitly.
	if len(empty) == 0 {
		return nil
	}
	updates := map[string]interface{}{}
	if user.RTN == "" {
		updates["rtn"] = nil
	}
	if user.Identity == "" {
		updates["identity"] = nil
	}
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error
}

// UpdateCreditScore updates only the credit score and timestamp so background
// recalculation cannot overwrite unrelated user fields.
func (r *userRepository) UpdateCreditScore(ctx context.Context, userID uint, score int) error {
	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"credit_score": score,
			"updated_at":   time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *userRepository) SetRecoveryCode(ctx context.Context, userID uint, code string, sentAt time.Time) error {
	sentAt = sentAt.UTC()
	u := &models.User{
		RecoveryCode:       &code,
		RecoveryCodeSentAt: &sentAt,
	}
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Select("RecoveryCode", "RecoveryCodeSentAt").
		Updates(u).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.User{}, id).Error
}

func (r *userRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("discarded_at", gorm.Expr("NOW()")).Error
}

func (r *userRepository) Restore(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("discarded_at", nil).Error
}

func (r *userRepository) List(ctx context.Context, query *ListQuery) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	db := r.db.WithContext(ctx).Model(&models.User{}).Where("discarded_at IS NULL")

	// Apply search
	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Where("full_name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR identity ILIKE ?",
			search, search, search, search)
	}

	// Apply role filter
	if query.Filters["role"] != "" {
		db = db.Where("role = ?", query.Filters["role"])
	}

	// Apply status filter
	if query.Filters["status"] != "" {
		db = db.Where("status = ?", query.Filters["status"])
	}

	// Historical clients are a subset of role=user. The flag lets the admin
	// Users page filter them without a second endpoint.
	if query.Filters["historical"] != "" {
		db = db.Where("historical_client = ?", true)
	}

	// Filter by creator (e.g. users created by a specific seller)
	if query.Filters["created_by"] != "" {
		db = db.Where("created_by = ?", query.Filters["created_by"])
	}

	// Count total
	db.Count(&total)

	// Apply sorting with allowlist to prevent SQL injection
	allowedSortColumns := map[string]bool{
		"id":           true,
		"email":        true,
		"full_name":    true,
		"phone":        true,
		"role":         true,
		"status":       true,
		"identity":     true,
		"rtn":          true,
		"created_at":   true,
		"updated_at":   true,
		"credit_score": true,
	}

	if query.SortBy != "" && allowedSortColumns[query.SortBy] {
		sortDir := "ASC"
		if query.SortDir == "desc" {
			sortDir = "DESC"
		}
		db = db.Order(fmt.Sprintf("users.%s %s", query.SortBy, sortDir))
	} else {
		db = db.Order("users.created_at DESC")
	}

	// Apply pagination
	if query.PerPage > 0 {
		db = db.Offset((query.Page - 1) * query.PerPage).Limit(query.PerPage)
	}

	err := db.Find(&users).Error
	return users, total, err
}

func (r *userRepository) FindAdmins(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("role = ? AND status = ? AND discarded_at IS NULL", models.RoleAdmin, models.StatusActive).
		Find(&users).Error
	return users, err
}

func (r *userRepository) FindAll(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("discarded_at IS NULL").
		Find(&users).Error
	return users, err
}

// ListQuery represents common query parameters
type ListQuery struct {
	Page    int
	PerPage int
	Search  string
	SortBy  string
	SortDir string
	Filters map[string]string
}

// NewListQuery creates a ListQuery with defaults
func NewListQuery() *ListQuery {
	return &ListQuery{
		Page:    1,
		PerPage: 20,
		Filters: make(map[string]string),
	}
}
