package http

import bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"

func registerProto(request RegisterRequest) *bookstorev1.RegisterRequest {
	return &bookstorev1.RegisterRequest{
		Email:       request.Email,
		Password:    request.Password,
		DisplayName: request.DisplayName,
	}
}

func loginProto(request LoginRequest) *bookstorev1.LoginRequest {
	return &bookstorev1.LoginRequest{Email: request.Email, Password: request.Password}
}

func googleLoginProto(request GoogleLoginRequest) *bookstorev1.GoogleLoginRequest {
	return &bookstorev1.GoogleLoginRequest{
		Credential:    request.Credential,
		CreateAccount: request.CreateAccount,
		Nonce:         request.State,
	}
}

func facebookLoginProto(request FacebookLoginRequest) *bookstorev1.FacebookLoginRequest {
	return &bookstorev1.FacebookLoginRequest{
		AccessToken:   request.AccessToken,
		CreateAccount: request.CreateAccount,
	}
}

func updateProfileProto(id string, request UpdateProfileRequest) *bookstorev1.UpdateProfileRequest {
	return &bookstorev1.UpdateProfileRequest{Id: id, DisplayName: request.DisplayName}
}

func createBookProto(request BookRequest) *bookstorev1.CreateBookRequest {
	return &bookstorev1.CreateBookRequest{
		Title:      request.Title,
		Author:     request.Author,
		Isbn:       request.ISBN,
		PriceCents: request.PriceCents,
		Stock:      request.Stock,
		SellerId:   request.SellerID,
	}
}

func updateBookProto(id string, request BookRequest) *bookstorev1.UpdateBookRequest {
	return &bookstorev1.UpdateBookRequest{
		Id:         id,
		Title:      request.Title,
		Author:     request.Author,
		Isbn:       request.ISBN,
		PriceCents: request.PriceCents,
		Stock:      request.Stock,
		SellerId:   request.SellerID,
	}
}

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

func usersJSON(users []*bookstorev1.User) []UserResponse {
	result := make([]UserResponse, 0, len(users))
	for _, user := range users {
		result = append(result, userJSON(user))
	}
	return result
}

func bookJSON(book *bookstorev1.Book) BookResponse {
	return BookResponse{
		ID:         book.GetId(),
		Title:      book.GetTitle(),
		Author:     book.GetAuthor(),
		ISBN:       book.GetIsbn(),
		PriceCents: book.GetPriceCents(),
		Stock:      book.GetStock(),
		SellerID:   book.GetSellerId(),
		CreatedAt:  book.GetCreatedAt(),
		UpdatedAt:  book.GetUpdatedAt(),
	}
}

func cartItemJSON(item *bookstorev1.CartItem) CartItemResponse {
	return CartItemResponse{
		ID: item.GetId(), BookID: item.GetBookId(), Quantity: item.GetQuantity(),
		CreatedAt: item.GetCreatedAt(), UpdatedAt: item.GetUpdatedAt(),
	}
}

func cartItemsJSON(items []*bookstorev1.CartItem) []CartItemResponse {
	result := make([]CartItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, cartItemJSON(item))
	}
	return result
}

func orderJSON(order *bookstorev1.Order) OrderResponse {
	items := make([]OrderItemResponse, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		items = append(items, OrderItemResponse{
			ID: item.GetId(), BookID: item.GetBookId(), SellerID: item.GetSellerId(), Title: item.GetTitle(),
			UnitPriceCents: item.GetUnitPriceCents(), Quantity: item.GetQuantity(),
			SubtotalCents: item.GetSubtotalCents(),
		})
	}
	return OrderResponse{
		ID: order.GetId(), Status: order.GetStatus(), TotalCents: order.GetTotalCents(),
		Currency: order.GetCurrency(), Items: items, PaymentID: order.GetPaymentId(),
		FailureReason: order.GetFailureReason(), CreatedAt: order.GetCreatedAt(), UpdatedAt: order.GetUpdatedAt(),
		ReservationExpiresAt: order.GetReservationExpiresAt(),
	}
}

func ordersJSON(orders []*bookstorev1.Order) []OrderResponse {
	result := make([]OrderResponse, 0, len(orders))
	for _, order := range orders {
		result = append(result, orderJSON(order))
	}
	return result
}

func paymentJSON(payment *bookstorev1.Payment) PaymentResponse {
	return PaymentResponse{
		ID: payment.GetId(), OrderID: payment.GetOrderId(), Status: payment.GetStatus(),
		AmountCents: payment.GetAmountCents(), PlatformFeeCents: payment.GetPlatformFeeCents(),
		Currency: payment.GetCurrency(), FailureReason: payment.GetFailureReason(),
		Provider: payment.GetProvider(), ProviderTransactionID: payment.GetProviderTransactionId(),
		CheckoutURL: payment.GetCheckoutUrl(), ExpiresAt: payment.GetExpiresAt(), PaidAt: payment.GetPaidAt(),
		CreatedAt: payment.GetCreatedAt(), UpdatedAt: payment.GetUpdatedAt(),
	}
}

func walletJSON(wallet *bookstorev1.Wallet) WalletResponse {
	return WalletResponse{
		ID: wallet.GetId(), OwnerID: wallet.GetOwnerId(), BalanceCents: wallet.GetBalanceCents(),
		Currency: wallet.GetCurrency(), CreatedAt: wallet.GetCreatedAt(), UpdatedAt: wallet.GetUpdatedAt(),
	}
}

func booksJSON(books []*bookstorev1.Book) []BookResponse {
	result := make([]BookResponse, 0, len(books))
	for _, book := range books {
		result = append(result, bookJSON(book))
	}
	return result
}
