package users_server

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/random"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func dial(t *testing.T) pb.UsersClient {
	t.Helper()
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return bufListener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	t.Cleanup(func() { conn.Close() })
	require.NoError(t, err)
	return pb.NewUsersClient(conn)
}

func authCtx(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(),
		"authorization", "bearer "+token,
	)
}

func randomUsername() string {
	return fmt.Sprintf("test_%s", random.RandomString(5))
}

func cleanTestUser(t *testing.T, username string) {
	t.Helper()
	var userID string
	err := testPool.QueryRow(context.Background(),
		`SELECT id::text FROM users WHERE username = $1`, username,
	).Scan(&userID)
	if err == nil && userID != "" {
		testPool.Exec(context.Background(), `DELETE FROM "outbox_events" WHERE aggregate_id = $1::uuid`, userID)
	}
	testPool.Exec(context.Background(), `DELETE FROM "refresh_tokens" WHERE username = $1`, username)
	testPool.Exec(context.Background(), `DELETE FROM users WHERE username = $1`, username)
}

func registerTestUser(t *testing.T, client pb.UsersClient) (username, password, accessToken, refreshToken string) {
	t.Helper()
	username = randomUsername()
	password = random.RandomString(10)

	res, err := client.Register(context.Background(), &pb.RegisterReq{
		Username: username,
		Password: password,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)

	t.Cleanup(func() { cleanTestUser(t, username) })
	return username, password, res.AccessToken, res.RefreshToken
}

func loginTestUser(t *testing.T, client pb.UsersClient, username, password string) (accessToken, refreshToken string) {
	t.Helper()
	res, err := client.Login(context.Background(), &pb.LoginReq{
		Username: username,
		Password: password,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)
	return res.AccessToken, res.RefreshToken
}

func createTestAdmin(t *testing.T, client pb.UsersClient) (username, password, accessToken string) {
	t.Helper()
	username = randomUsername()
	password = random.RandomString(10)

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	pool, err := pgxpool.New(context.Background(), testCfg.DBSource)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(context.Background(),
		`INSERT INTO users (role, password, username) VALUES ($1, $2, $3)`,
		"ADMIN", string(hashed), username,
	)
	require.NoError(t, err)
	t.Cleanup(func() { cleanTestUser(t, username) })

	accessToken, _ = loginTestUser(t, client, username, password)
	return username, password, accessToken
}

func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, want, st.Code())
}
