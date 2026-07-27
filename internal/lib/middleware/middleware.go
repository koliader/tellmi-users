package middleware

import "github.com/koliader/tellmi-users/internal/lib/token"

type GrpcMiddleware struct {
	tokenMaker token.Maker
}

func NewMiddleware(tokenMaker token.Maker) *GrpcMiddleware {
	return &GrpcMiddleware{
		tokenMaker: tokenMaker,
	}
}
