package db_err

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestErrorCodeUniqueViolation(t *testing.T) {
	code := ErrorCode(&pgconn.PgError{Code: UniqueViolation})
	require.Equal(t, UniqueViolation, code)
}

func TestErrorCodeForeignKeyViolation(t *testing.T) {
	code := ErrorCode(&pgconn.PgError{Code: ForeignKeyViolation})
	require.Equal(t, ForeignKeyViolation, code)
}

func TestErrorCodeNonExistingError(t *testing.T) {
	code := ErrorCode(errors.New("some new error"))
	require.Empty(t, code)
}

func TestErrorCodeNilError(t *testing.T) {
	code := ErrorCode(nil)
	require.Empty(t, code)
}
