package users_server

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/koliader/tellmi-sdk/config"
	"github.com/koliader/tellmi-sdk/middleware"
	"github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/token"
	users_service "github.com/koliader/tellmi-users/internal/services/users"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testConfig struct {
	DBSource             string        `mapstructure:"DB_SOURCE"`
	TokenKey             string        `mapstructure:"TOKEN_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
}

type noopSender struct{}

func (n *noopSender) SendMessage(queue string, body []byte) error { return nil }

var (
	testStore   db.Store
	testPool    *pgxpool.Pool
	testCfg     testConfig
	bufListener *bufconn.Listener
)

func TestMain(m *testing.M) {
	err := config.LoadConfig("../../../..", &testCfg)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	testPool, err = pgxpool.New(context.Background(), testCfg.DBSource)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	defer testPool.Close()

	testStore = db.NewStore(testPool)

	bufListener = bufconn.Listen(1024 * 1024)
	go startTestServer()

	os.Exit(m.Run())
}

func startTestServer() {
	tokenMaker, _ := token.NewJWTMaker(testCfg.TokenKey)
	mw := middleware.NewMiddleware(tokenMaker)

	svc, _ := users_service.NewService(
		tokenMaker,
		testCfg.AccessTokenDuration,
		testCfg.RefreshTokenDuration,
		testStore,
		&noopSender{},
	)

	srv := NewServer(svc, mw)
	lis := grpc.NewServer()
	pb.RegisterUsersServer(lis, srv)
	lis.Serve(bufListener)
}

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

func cleanTestUser(t *testing.T, username string) {
	t.Helper()
	testPool.Exec(context.Background(), `DELETE FROM users WHERE username = $1`, username)
}

func TestIntegration_Register(t *testing.T) {
	client := dial(t)
	username := fmt.Sprintf("test_%d", time.Now().UnixNano())
	defer cleanTestUser(t, username)

	res, err := client.Register(context.Background(), &pb.RegisterReq{
		Username: username,
		Password: "secret123",
	})

	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)

	// Verify real JWT by decoding payload
	maker, err := token.NewJWTMaker(testCfg.TokenKey)
	require.NoError(t, err)
	payload, err := maker.VerifyToken(res.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "USER", payload.Role)

	// Verify user actually exists in DB with matching UUID
	var (
		dbID       string
		dbUsername string
	)
	err = testPool.QueryRow(context.Background(),
		`SELECT id::text, username FROM users WHERE username = $1`, username,
	).Scan(&dbID, &dbUsername)
	require.NoError(t, err)
	require.Equal(t, dbID, payload.ID.String())
	require.Equal(t, username, dbUsername)

	// Verify refresh token stored in DB
	var tokenCount int
	err = testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM "refresh_tokens" WHERE username = $1`, username,
	).Scan(&tokenCount)
	require.NoError(t, err)
	require.Equal(t, 1, tokenCount)
}

func TestIntegration_ListUsersAsUserFails(t *testing.T) {
	client := dial(t)
	username := fmt.Sprintf("user_%d", time.Now().UnixNano())
	defer cleanTestUser(t, username)

	_, err := client.Register(context.Background(), &pb.RegisterReq{
		Username: username,
		Password: "pass123",
	})
	require.NoError(t, err)

	loginRes, err := client.Login(context.Background(), &pb.LoginReq{
		Username: username,
		Password: "pass123",
	})
	require.NoError(t, err)

	// USER role should NOT be able to ListUsers (requires ADMIN)
	_, err = client.ListUsers(authCtx(loginRes.AccessToken), &pb.Empty{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}
