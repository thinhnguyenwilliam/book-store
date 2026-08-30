package grpcclient

import (
	"context"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
)

type RecipientResolver struct{ client bookstorev1.UserServiceClient }

func NewRecipientResolver(client bookstorev1.UserServiceClient) *RecipientResolver {
	return &RecipientResolver{client: client}
}

func (r *RecipientResolver) Resolve(ctx context.Context, userID string) (domain.Recipient, error) {
	user, err := r.client.GetProfile(ctx, &bookstorev1.GetProfileRequest{Id: userID})
	if err != nil {
		return domain.Recipient{}, err
	}
	return domain.Recipient{UserID: user.GetId(), Email: user.GetEmail(), DisplayName: user.GetDisplayName()}, nil
}
