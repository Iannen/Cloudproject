package models

import (
	"context"
)

type DomainHandler func(ctx context.Context, body []byte) (string, error)
