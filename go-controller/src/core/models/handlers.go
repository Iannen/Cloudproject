package models

import (
	"context"
)

type PayloadHandler[T any] func(ctx context.Context, payload T) (string, error)
type ActionHandler func(ctx context.Context) (string, error)
