package db_tests

import (
	"context"
	"testing"
	"time"

	"github.com/koliader/tellmi-users/internal/lib/random"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
)

func createRandomRefreshToken(t *testing.T, username string) db.RefreshToken {
	t.Helper()

	rt, err := testStore.CreateRefreshToken(context.Background(), db.CreateRefreshTokenParams{
		Token:     random.RandomString(32),
		Username:  username,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	require.NotEmpty(t, rt)

	require.Equal(t, username, rt.Username)
	require.NotEmpty(t, rt.Token)
	require.True(t, rt.ExpiresAt.After(time.Now()))

	return rt
}

func TestCreateRefreshToken(t *testing.T) {
	user := createRandomUser(t)
	createRandomRefreshToken(t, user.Username)
}

func TestGetRefreshToken(t *testing.T) {
	user := createRandomUser(t)
	created := createRandomRefreshToken(t, user.Username)

	fetched, err := testStore.GetRefreshToken(context.Background(), created.Token)
	require.NoError(t, err)
	require.NotEmpty(t, fetched)

	require.Equal(t, created.Token, fetched.Token)
	require.Equal(t, created.Username, fetched.Username)
	require.Equal(t, created.ExpiresAt.Unix(), fetched.ExpiresAt.Unix())
}

func TestGetRefreshTokenNotFound(t *testing.T) {
	_, err := testStore.GetRefreshToken(context.Background(), "nonexistent_token")
	require.Error(t, err)
}

func TestDeleteRefreshToken(t *testing.T) {
	user := createRandomUser(t)
	created := createRandomRefreshToken(t, user.Username)

	err := testStore.DeleteRefreshToken(context.Background(), created.Token)
	require.NoError(t, err)

	_, err = testStore.GetRefreshToken(context.Background(), created.Token)
	require.Error(t, err)
}

func TestDeleteRefreshTokenNotFound(t *testing.T) {
	err := testStore.DeleteRefreshToken(context.Background(), "nonexistent_token")
	require.NoError(t, err)
}

func TestDeleteRefreshTokensByUsername(t *testing.T) {
	user := createRandomUser(t)
	rt1 := createRandomRefreshToken(t, user.Username)
	rt2 := createRandomRefreshToken(t, user.Username)

	err := testStore.DeleteRefreshTokensByUsername(context.Background(), user.Username)
	require.NoError(t, err)

	_, err = testStore.GetRefreshToken(context.Background(), rt1.Token)
	require.Error(t, err)

	_, err = testStore.GetRefreshToken(context.Background(), rt2.Token)
	require.Error(t, err)
}

func TestDeleteRefreshTokensByUsernameNoTokens(t *testing.T) {
	err := testStore.DeleteRefreshTokensByUsername(context.Background(), "nonexistent_user")
	require.NoError(t, err)
}
