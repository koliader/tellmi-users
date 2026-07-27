package grpc_err

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorResponseNoArgs(t *testing.T) {
	err := ErrorResponse(codes.NotFound, "not found")
	st, ok := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
	require.Equal(t, "not found", st.Message())
	require.True(t, ok)
}

func TestErrorResponseWithArgs(t *testing.T) {
	err := ErrorResponse(codes.InvalidArgument, "invalid value", "42")
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
	require.Contains(t, st.Message(), "invalid value")
	require.Contains(t, st.Message(), "42")
	require.Equal(t, "invalid value: 42", st.Message())
}

func TestAuthError(t *testing.T) {
	err := AuthError(fmt.Errorf("token expired"))
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Contains(t, st.Message(), "authentication error")
	require.Contains(t, st.Message(), "token expired")
	require.Equal(t, "authentication error: token expired", st.Message())
}
