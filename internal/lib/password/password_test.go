package password

import (
	"github.com/koliader/tellmi-sdk/random"
	"testing"

	"github.com/stretchr/testify/require"
)

func createHashedPassword(t *testing.T, psswrd string) string {
	t.Helper()

	hashPassword, err := HashPassword(psswrd)
	require.NoError(t, err)
	require.NotEmpty(t, hashPassword)
	require.NotEqual(t, psswrd, hashPassword)
	return hashPassword
}

func TestHashPassword(t *testing.T) {
	createHashedPassword(t, random.RandomString(10))
}

func TestHashEmptyPassword(t *testing.T) {
	hashPassword, err := HashPassword("")
	require.Error(t, err)
	require.Empty(t, hashPassword)
}

func TestCheckPassword(t *testing.T) {
	psswrd := random.RandomString(10)
	hashPassword := createHashedPassword(t, psswrd)

	err := CheckPassword(hashPassword, psswrd)
	require.NoError(t, err)
}

func TestCheckWrongPassword(t *testing.T) {
	hashPassword := createHashedPassword(t, random.RandomString(10))

	err := CheckPassword(hashPassword, random.RandomString(10))
	require.Error(t, err)
}

func TestUniqueHash(t *testing.T) {
	psswrd := random.RandomString(10)
	hash1 := createHashedPassword(t, psswrd)
	hash2 := createHashedPassword(t, psswrd)

	require.NotEqual(t, hash1, hash2)
}
