package converter

import (
	"testing"

	"github.com/koliader/tellmi-users/internal/lib/random"
	"github.com/koliader/tellmi-users/internal/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
)

func convertOneUser(t *testing.T) *pb.User {
	t.Helper()

	user := db.User{
		ID:        random.RandomInt(1, 10),
		Role:      "USER",
		Password:  random.RandomString(10),
		Username:  random.RandomString(10),
		IsBlocked: random.RandomBool(),
	}
	converted := ConvertUser(user)
	require.Equal(t, user.ID, converted.Id)
	require.Equal(t, user.Role, converted.Role)
	require.Equal(t, user.IsBlocked, converted.IsBlocked)
	require.Equal(t, user.Username, converted.Username)
	return converted
}

func TestConvertUser(t *testing.T) {
	convertOneUser(t)
}

func TestConvertManyUsers(t *testing.T) {
	var users []db.User
	for range 10 {
		user := db.User{
			ID:        random.RandomInt(1, 10),
			Role:      "USER",
			Password:  random.RandomString(10),
			Username:  random.RandomString(10),
			IsBlocked: random.RandomBool(),
		}
		users = append(users, user)
	}

	converted := ConvertUsers(users)
	require.NotEmpty(t, converted)
	require.Equal(t, len(converted), 10)
	for _, user := range converted {
		require.NotEmpty(t, user)
	}
}
