package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidInput   = errors.New("invalid comment input")
	ErrNotFound       = errors.New("comment not found")
	ErrForbidden      = errors.New("comment action is forbidden")
	ErrParentMismatch = errors.New("parent comment belongs to another book")
	ErrMaxDepth       = errors.New("maximum comment reply depth reached")
	ErrNotEditable    = errors.New("comment is no longer editable")
)

const (
	StatusPublished  = "published"
	StatusHidden     = "hidden"
	StatusDeleted    = "deleted"
	MaxDepth         = int32(3)
	MaxContentLength = 2000
)

type Comment struct {
	ID, BookID, AuthorID, AuthorName string
	ParentID, RootID                 string
	Depth                            int32
	Content, Status                  string
	ReplyCount                       int64
	CreatedAt, UpdatedAt             time.Time
	DeletedAt                        time.Time
}

func NormalizeContent(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > MaxContentLength {
		return "", ErrInvalidInput
	}
	return value, nil
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}
type Page struct {
	Comments   []*Comment
	NextCursor string
	HasMore    bool
}
