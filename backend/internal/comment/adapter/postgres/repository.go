package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/domain"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type commentModel struct {
	ID, BookID, AuthorID, AuthorName string
	ParentID                         *string
	RootID                           string
	Depth                            int32
	Content, Status                  string
	ReplyCount                       int64 `gorm:"column:reply_count;->"`
	CreatedAt, UpdatedAt             time.Time
	DeletedAt                        *time.Time
}

func (r *Repository) Create(ctx context.Context, item *domain.Comment) error {
	record := toModel(item)
	if err := r.db.WithContext(ctx).Table("comments.comments").Create(&record).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Comment, error) {
	var record commentModel
	err := r.db.WithContext(ctx).Table("comments.comments").Where("id = ?", id).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find comment: %w", err)
	}
	return toDomain(record), nil
}

func (r *Repository) ListRoots(ctx context.Context, bookID string, limit int32, cursor *domain.Cursor) ([]*domain.Comment, error) {
	db := r.withReplyCount(ctx).Where("c.book_id = ? AND c.parent_id IS NULL", bookID).
		Order("c.created_at DESC, c.id DESC").Limit(int(limit))
	if cursor != nil {
		db = db.Where("(c.created_at, c.id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	return scanComments(db, "list root comments")
}

func (r *Repository) ListReplies(ctx context.Context, rootID string, limit int32, cursor *domain.Cursor) ([]*domain.Comment, error) {
	db := r.withReplyCount(ctx).Where("c.root_id = ? AND c.parent_id IS NOT NULL", rootID).
		Order("c.created_at ASC, c.id ASC").Limit(int(limit))
	if cursor != nil {
		db = db.Where("(c.created_at, c.id) > (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	return scanComments(db, "list comment replies")
}

func (r *Repository) withReplyCount(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("comments.comments AS c").Select("c.*, (SELECT COUNT(*) FROM comments.comments child WHERE child.parent_id = c.id) AS reply_count")
}

func scanComments(db *gorm.DB, operation string) ([]*domain.Comment, error) {
	var records []commentModel
	if err := db.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	items := make([]*domain.Comment, 0, len(records))
	for _, record := range records {
		items = append(items, toDomain(record))
	}
	return items, nil
}

func (r *Repository) Update(ctx context.Context, id, authorID, content string, now time.Time) (*domain.Comment, error) {
	result := r.db.WithContext(ctx).Table("comments.comments").Where("id = ? AND author_id = ? AND status = ?", id, authorID, domain.StatusPublished).
		Updates(map[string]any{"content": content, "updated_at": now})
	if result.Error != nil {
		return nil, fmt.Errorf("update comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrNotEditable
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) SoftDelete(ctx context.Context, id string, now time.Time) (*domain.Comment, error) {
	result := r.db.WithContext(ctx).Table("comments.comments").Where("id = ? AND status <> ?", id, domain.StatusDeleted).
		Updates(map[string]any{"status": domain.StatusDeleted, "deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return nil, fmt.Errorf("delete comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return r.FindByID(ctx, id)
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Moderate(ctx context.Context, id, status string, now time.Time) (*domain.Comment, error) {
	result := r.db.WithContext(ctx).Table("comments.comments").Where("id = ? AND status <> ?", id, domain.StatusDeleted).
		Updates(map[string]any{"status": status, "updated_at": now})
	if result.Error != nil {
		return nil, fmt.Errorf("moderate comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrNotEditable
	}
	return r.FindByID(ctx, id)
}

func toModel(item *domain.Comment) commentModel {
	var parentID *string
	if item.ParentID != "" {
		value := item.ParentID
		parentID = &value
	}
	return commentModel{ID: item.ID, BookID: item.BookID, AuthorID: item.AuthorID, AuthorName: item.AuthorName, ParentID: parentID, RootID: item.RootID, Depth: item.Depth, Content: item.Content, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toDomain(record commentModel) *domain.Comment {
	item := &domain.Comment{ID: record.ID, BookID: record.BookID, AuthorID: record.AuthorID, AuthorName: record.AuthorName, RootID: record.RootID, Depth: record.Depth, Content: record.Content, Status: record.Status, ReplyCount: record.ReplyCount, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if record.ParentID != nil {
		item.ParentID = *record.ParentID
	}
	if record.DeletedAt != nil {
		item.DeletedAt = *record.DeletedAt
	}
	return item
}
