package password

import (
	"context"
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/koliader/tellmi-sdk/random"
	"github.com/stretchr/testify/require"
)

func createHashedPassword(t *testing.T, psswrd string) string {
	t.Helper()

	hashPassword, err := HashPassword(context.Background(), psswrd)
	require.NoError(t, err)
	require.NotEmpty(t, hashPassword)
	require.NotEqual(t, psswrd, hashPassword)
	return hashPassword
}

func TestHashPassword(t *testing.T) {
	createHashedPassword(t, random.RandomString(10))
}

func TestHashEmptyPassword(t *testing.T) {
	hashPassword, err := HashPassword(context.Background(), "")
	require.Error(t, err)
	require.Empty(t, hashPassword)
}

func TestCheckPassword(t *testing.T) {
	psswrd := random.RandomString(10)
	hashPassword := createHashedPassword(t, psswrd)

	err := CheckPassword(context.Background(), hashPassword, psswrd)
	require.NoError(t, err)
}

func TestCheckWrongPassword(t *testing.T) {
	hashPassword := createHashedPassword(t, random.RandomString(10))

	err := CheckPassword(context.Background(), hashPassword, random.RandomString(10))
	require.Error(t, err)
}

func TestUniqueHash(t *testing.T) {
	psswrd := random.RandomString(10)
	hash1 := createHashedPassword(t, psswrd)
	hash2 := createHashedPassword(t, psswrd)

	require.NotEqual(t, hash1, hash2)
}

func TestHashFormat(t *testing.T) {
	hash := createHashedPassword(t, random.RandomString(10))

	require.True(t, strings.HasPrefix(hash, "$argon2id$v=19$"), "hash should use argon2id PHC format: %s", hash)
}

func TestCheckMalformedHash(t *testing.T) {
	err := CheckPassword(context.Background(), "not-a-valid-hash", random.RandomString(10))
	require.Error(t, err)
}

func TestCheckEmptyHash(t *testing.T) {
	err := CheckPassword(context.Background(), "", random.RandomString(10))
	require.Error(t, err)
}

func TestCheckNonDefaultParams(t *testing.T) {
	params := *argon2id.DefaultParams
	params.Memory = 8 * 1024

	psswrd := random.RandomString(10)
	hash, err := argon2id.CreateHash(psswrd, &params)
	require.NoError(t, err)

	err = CheckPassword(context.Background(), hash, psswrd)
	require.NoError(t, err)

	err = CheckPassword(context.Background(), hash, random.RandomString(10))
	require.Error(t, err)
}

func BenchmarkHashPassword(b *testing.B) {
	psswrd := random.RandomString(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HashPassword(context.Background(), psswrd)
		if err != nil {
			b.Fatal(err)
		}
	}
}
