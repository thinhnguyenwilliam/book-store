package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid search input")
	ErrUnavailable  = errors.New("book search is temporarily unavailable")
)

type BookDocument struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Author          string    `json:"author"`
	ISBN            string    `json:"isbn"`
	PriceCents      int64     `json:"price_cents"`
	Stock           int32     `json:"stock"`
	SellerID        string    `json:"seller_id,omitempty"`
	PopularityScore float64   `json:"popularity_score"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Filters struct {
	MinPriceCents int64
	MaxPriceCents int64
	InStock       *bool
	SellerID      string
	Author        string
}

type Request struct {
	Query       string
	Limit       int
	Filters     Filters
	Sort        string
	SearchAfter []any
}

type Hit struct {
	Book       BookDocument
	Score      float64
	Highlights map[string]string
	Sort       []any
}

type Result struct {
	Hits   []Hit
	Total  int64
	TookMS int64
}
