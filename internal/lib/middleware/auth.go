package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/koliader/tellmi-users/internal/lib/token"
	"google.golang.org/grpc/metadata"
)

const (
	authorizationHeader = "authorization"
	authBearer          = "bearer"
	admin               = "ADMIN"
)

func (m *Middleware) AuthorizeUser(ctx context.Context) (*token.Payload, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing metadata")
	}
	values := md.Get(authorizationHeader)
	if len(values) == 0 {
		return nil, fmt.Errorf("missing auth header")
	}
	authHeader := values[0]
	fields := strings.Fields(authHeader)
	if len(fields) < 1 {
		return nil, fmt.Errorf("invalid auth header format")
	}
	token := fields[0]
	payload, err := m.tokenMaker.VerifyToken(token)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
