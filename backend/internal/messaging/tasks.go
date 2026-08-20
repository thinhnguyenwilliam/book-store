package messaging

const (
	EventAccountRegistered = "account.registered"
	EventAccountDeleted    = "account.deleted"
)

type AccountRegisteredPayload struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type AccountDeletedPayload struct {
	UserID    string `json:"user_id"`
	DeletedAt string `json:"deleted_at"`
}
