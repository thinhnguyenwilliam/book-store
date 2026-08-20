package messaging

const EventAccountRegistered = "account.registered"

type AccountRegisteredPayload struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}
