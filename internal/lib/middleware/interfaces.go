package middleware

import (
	"context"

	"github.com/koliader/tellmi-users/internal/lib/token"
)

type Middleware interface {
	AuthorizeUser(ctx context.Context) (*token.Payload, error)
	AuthorizeAdmin(ctx context.Context) (*token.Payload, error)
}
