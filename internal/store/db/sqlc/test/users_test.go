package db_tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/koliader/tellmi-sdk/random"
	"github.com/koliader/tellmi-users/internal/lib/password"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
)

func createRandomUser(t *testing.T) db.User {
	t.Helper()

	hashedPassword, err := password.HashPassword(context.Background(), random.RandomString(8))
	require.NoError(t, err)

	user, err := testStore.CreateUser(context.Background(), db.CreateUserParams{
		Password: hashedPassword,
		Username: random.RandomString(10),
	})
	require.NoError(t, err)
	require.NotEmpty(t, user)

	require.NotEmpty(t, user.ID)
	require.Equal(t, hashedPassword, user.Password)
	require.Equal(t, false, user.IsBlocked)
	require.Equal(t, "USER", user.Role)

	return user
}

func TestCreateUser(t *testing.T) {
	createRandomUser(t)
}

func TestCreateUserDuplicateUsername(t *testing.T) {
	user := createRandomUser(t)

	hashedPassword, err := password.HashPassword(context.Background(), random.RandomString(8))
	require.NoError(t, err)

	_, err = testStore.CreateUser(context.Background(), db.CreateUserParams{
		Password: hashedPassword,
		Username: user.Username,
	})
	require.Error(t, err)
}

func TestGetUserById(t *testing.T) {
	created := createRandomUser(t)

	fetched, err := testStore.GetUserById(context.Background(), created.ID)
	require.NoError(t, err)
	require.NotEmpty(t, fetched)

	require.Equal(t, created.ID, fetched.ID)
	require.Equal(t, created.Username, fetched.Username)
	require.Equal(t, created.Password, fetched.Password)
	require.Equal(t, created.Role, fetched.Role)
	require.Equal(t, created.IsBlocked, fetched.IsBlocked)
}

func TestGetUserByIdNotFound(t *testing.T) {
	_, err := testStore.GetUserById(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestGetUserByUsername(t *testing.T) {
	created := createRandomUser(t)

	fetched, err := testStore.GetUserByUsername(context.Background(), created.Username)
	require.NoError(t, err)
	require.NotEmpty(t, fetched)

	require.Equal(t, created.ID, fetched.ID)
	require.Equal(t, created.Username, fetched.Username)
	require.Equal(t, created.Password, fetched.Password)
	require.Equal(t, created.Role, fetched.Role)
	require.Equal(t, created.IsBlocked, fetched.IsBlocked)
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	_, err := testStore.GetUserByUsername(context.Background(), "nonexistent_user")
	require.Error(t, err)
}

func TestListUsers(t *testing.T) {
	for range 3 {
		createRandomUser(t)
	}

	users, err := testStore.ListUsers(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, users)
	require.GreaterOrEqual(t, len(users), 3)
}

func TestUpdateUser(t *testing.T) {
	created := createRandomUser(t)

	newUsername := random.RandomString(10)
	updated, err := testStore.UpdateUser(context.Background(), db.UpdateUserParams{
		ID:       created.ID,
		Username: newUsername,
	})
	require.NoError(t, err)
	require.NotEmpty(t, updated)

	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, newUsername, updated.Username)
	require.Equal(t, created.Password, updated.Password)
	require.Equal(t, created.Role, updated.Role)
	require.Equal(t, created.IsBlocked, updated.IsBlocked)
}

func TestUpdateUserNotFound(t *testing.T) {
	_, err := testStore.UpdateUser(context.Background(), db.UpdateUserParams{
		ID:       uuid.New(),
		Username: random.RandomString(10),
	})
	require.Error(t, err)
}
