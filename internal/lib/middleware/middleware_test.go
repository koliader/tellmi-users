package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/koliader/tellmi-users/internal/lib/token"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

type mockMaker struct {
	payload *token.Payload
	err     error
}

// to pass Maker interface
func (m *mockMaker) CreateToken(username, role string, duration time.Duration) (string, error) {
	return "", nil
}

func (m *mockMaker) VerifyToken(tokenString string) (*token.Payload, error) {
	return m.payload, m.err
}

// helper
func newCtxWithAuth(header string) context.Context {
	md := metadata.Pairs("authorization", header)
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestAuthorizeUserValid(t *testing.T) {
	m := NewMiddleware(&mockMaker{
		payload: &token.Payload{Username: "alice", Role: "USER"},
	})

	payload, err := m.AuthorizeUser(newCtxWithAuth("bearer some-token"))
	require.NoError(t, err)
	require.Equal(t, "alice", payload.Username)
}

func TestAuthorizeUserNoMetadata(t *testing.T) {
	m := NewMiddleware(&mockMaker{})

	_, err := m.AuthorizeUser(context.Background())
	require.Error(t, err)
}

func TestAuthorizeUserMissingHeader(t *testing.T) {
	m := NewMiddleware(&mockMaker{})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())

	_, err := m.AuthorizeUser(ctx)
	require.Error(t, err)
}

func TestAuthorizeUserInvalidToken(t *testing.T) {
	m := NewMiddleware(&mockMaker{err: token.ErrInvalidToken})

	_, err := m.AuthorizeUser(newCtxWithAuth("bearer bad-token"))
	require.Error(t, err)
}

func TestAuthorizeAdminValid(t *testing.T) {
	m := NewMiddleware(&mockMaker{
		payload: &token.Payload{Username: "bob", Role: "ADMIN"},
	})

	payload, err := m.AuthorizeAdmin(newCtxWithAuth("bearer some-token"))
	require.NoError(t, err)
	require.Equal(t, "bob", payload.Username)
}

func TestAuthorizeAdminUserRoleRejected(t *testing.T) {
	m := NewMiddleware(&mockMaker{
		payload: &token.Payload{Username: "alice", Role: "USER"},
	})

	_, err := m.AuthorizeAdmin(newCtxWithAuth("bearer some-token"))
	require.Error(t, err)
}

func TestAuthorizeUserWithRealJWT(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz123456"
	maker, err := token.NewJWTMaker(key)
	require.NoError(t, err)

	m := NewMiddleware(maker)

	tokenStr, err := maker.CreateToken("alice", "USER", time.Minute)
	require.NoError(t, err)

	payload, err := m.AuthorizeUser(newCtxWithAuth("bearer " + tokenStr))
	require.NoError(t, err)
	require.Equal(t, "alice", payload.Username)
	require.Equal(t, "USER", payload.Role)
}

func TestAuthorizeAdminWithRealJWTWrongRole(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz123456"
	maker, err := token.NewJWTMaker(key)
	require.NoError(t, err)

	m := NewMiddleware(maker)

	tokenStr, err := maker.CreateToken("alice", "USER", time.Minute)
	require.NoError(t, err)

	_, err = m.AuthorizeAdmin(newCtxWithAuth("bearer " + tokenStr))
	require.Error(t, err)
}
