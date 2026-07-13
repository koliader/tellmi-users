package grpc_err

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ErrorResponse(code codes.Code, msg string, a ...any) error {
	if len(a) > 0 {
		return status.Errorf(code, "%s: %v", msg, a)
	}
	return status.Errorf(code, msg)
}

func AuthError(err error) error {
	return status.Errorf(codes.Unauthenticated, "authentication error: %v", err)
}
