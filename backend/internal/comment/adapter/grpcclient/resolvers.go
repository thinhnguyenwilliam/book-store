package grpcclient

import (
	"context"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

type BookResolver struct{ client bookstorev1.BookServiceClient }

func NewBookResolver(client bookstorev1.BookServiceClient) *BookResolver {
	return &BookResolver{client: client}
}
func (r *BookResolver) Exists(ctx context.Context, id string) error {
	_, err := r.client.GetBook(ctx, &bookstorev1.GetBookRequest{Id: id})
	return err
}

type AuthorResolver struct{ client bookstorev1.UserServiceClient }

func NewAuthorResolver(client bookstorev1.UserServiceClient) *AuthorResolver {
	return &AuthorResolver{client: client}
}
func (r *AuthorResolver) DisplayName(ctx context.Context, id string) (string, error) {
	profile, err := r.client.GetProfile(ctx, &bookstorev1.GetProfileRequest{Id: id})
	if err != nil {
		return "", err
	}
	if profile.GetDisplayName() != "" {
		return profile.GetDisplayName(), nil
	}
	return profile.GetEmail(), nil
}
