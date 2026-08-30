package graphql

import (
	"context"
	"fmt"
	"net/http"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/gateway/graphql/generated"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type ServerConfig struct {
	MaxComplexity        int
	MaxDepth             int
	ParserTokenLimit     int
	IntrospectionEnabled bool
}

func NewServer(
	config ServerConfig,
	books bookstorev1.BookServiceClient,
	users bookstorev1.UserServiceClient,
	orders bookstorev1.OrderServiceClient,
	payments bookstorev1.PaymentServiceClient,
	comments bookstorev1.CommentServiceClient,
) (http.Handler, error) {
	if config.MaxComplexity < 1 || config.MaxDepth < 1 || config.ParserTokenLimit < 1 {
		return nil, fmt.Errorf("invalid GraphQL server limits")
	}
	resolver := &Resolver{Books: books, Users: users, Orders: orders, Payments: payments, Comments: comments}
	server := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	server.AddTransport(transport.POST{})
	server.SetQueryCache(lru.New[*ast.QueryDocument](100))
	server.SetParserTokenLimit(config.ParserTokenLimit)
	server.SetDisableSuggestion(true)
	server.Use(extension.FixedComplexityLimit(config.MaxComplexity))
	if config.IntrospectionEnabled {
		server.Use(extension.Introspection{})
	}
	server.AroundOperations(depthLimit(config.MaxDepth))
	return server, nil
}

func depthLimit(maxDepth int) gql.OperationMiddleware {
	return func(ctx context.Context, next gql.OperationHandler) gql.ResponseHandler {
		operation := gql.GetOperationContext(ctx).Operation
		if operation != nil && selectionDepth(operation.SelectionSet, 0) > maxDepth {
			return gql.OneShot(&gql.Response{Errors: gqlerror.List{&gqlerror.Error{
				Message:    "query depth exceeds configured limit",
				Extensions: map[string]any{"code": "QUERY_TOO_DEEP"},
			}}})
		}
		return next(ctx)
	}
}

func selectionDepth(selections ast.SelectionSet, current int) int {
	maximum := current
	for _, selection := range selections {
		switch item := selection.(type) {
		case *ast.Field:
			depth := current + 1
			if nested := selectionDepth(item.SelectionSet, depth); nested > depth {
				depth = nested
			}
			if depth > maximum {
				maximum = depth
			}
		case *ast.InlineFragment:
			if depth := selectionDepth(item.SelectionSet, current); depth > maximum {
				maximum = depth
			}
		case *ast.FragmentSpread:
			if item.Definition != nil {
				if depth := selectionDepth(item.Definition.SelectionSet, current); depth > maximum {
					maximum = depth
				}
			}
		}
	}
	return maximum
}
