package middleware

import "github.com/koliader/tellmi-users/internal/lib/token"

type Middleware struct {
	tokenMaker token.Maker
}

func NewMiddleware(tokenMaker token.Maker) *Middleware {
	return &Middleware{
		tokenMaker: tokenMaker,
	}
}
