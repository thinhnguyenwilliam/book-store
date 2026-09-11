package domain

import "context"

type actorKey struct{}

// ContextWithActor carries a verified actor for transactional authorization.
func ContextWithActor(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, actorKey{}, id)
}
func ActorFromContext(ctx context.Context) string { id, _ := ctx.Value(actorKey{}).(string); return id }
