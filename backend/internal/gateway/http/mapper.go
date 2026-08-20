package http

import bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"

func authJSON(response *bookstorev1.AuthResponse) AuthResponse {
	return AuthResponse{
		AccessToken: response.GetAccessToken(),
		TokenType:   "Bearer",
		UserID:      response.GetUserId(),
		ExpiresIn:   response.GetExpiresIn(),
	}
}

func userJSON(user *bookstorev1.User) UserResponse {
	return UserResponse{
		ID:          user.GetId(),
		Email:       user.GetEmail(),
		DisplayName: user.GetDisplayName(),
		CreatedAt:   user.GetCreatedAt(),
		UpdatedAt:   user.GetUpdatedAt(),
	}
}

func bookJSON(book *bookstorev1.Book) BookResponse {
	return BookResponse{
		ID:         book.GetId(),
		Title:      book.GetTitle(),
		Author:     book.GetAuthor(),
		ISBN:       book.GetIsbn(),
		PriceCents: book.GetPriceCents(),
		Stock:      book.GetStock(),
		CreatedAt:  book.GetCreatedAt(),
		UpdatedAt:  book.GetUpdatedAt(),
	}
}
