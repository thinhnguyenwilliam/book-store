package domain

import "time"

type OrderReport struct {
	From                       time.Time
	To                         time.Time
	TotalOrders                int64
	ConfirmedOrders            int64
	CancelledOrders            int64
	PaymentAttempts            int64
	PaymentSucceeded           int64
	PaymentFailed              int64
	StockReservationFailed     int64
	PaymentSuccessRate         float64
	AverageConfirmationSeconds float64
	Daily                      []DailyOrderMetric
	LastEventAt                *time.Time
}

type DailyOrderMetric struct {
	Date      time.Time
	Created   int64
	Confirmed int64
	Cancelled int64
}

type CustomerActivityReport struct {
	From                time.Time
	To                  time.Time
	TotalEvents         int64
	UniqueActors        int64
	AbandonedCarts      int64
	ViewToCartRate      float64
	CartToCheckoutRate  float64
	CheckoutToOrderRate float64
	EventCounts         []EventCount
	TopBooks            []BookActivityMetric
	LastEventAt         *time.Time
}

type EventCount struct {
	EventType string
	Count     int64
}

type BookActivityMetric struct {
	BookID   string
	Views    int64
	CartAdds int64
	Comments int64
	Score    float64
}

type RelatedBook struct {
	BookID       string
	SharedActors int64
	Score        float64
}
