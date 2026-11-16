package trip

import (
	"context"

	"github.com/google/uuid"
)

type tripIDKey struct{}

func WithTripID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, tripIDKey{}, id)
}

func TripIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(tripIDKey{}).(uuid.UUID)
	return id, ok
}
