package users_server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/koliader/tellmi-sdk/config"
	"github.com/koliader/tellmi-sdk/middleware"
	"github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/random"
	"github.com/koliader/tellmi-sdk/token"
	users_service "github.com/koliader/tellmi-users/internal/services/users"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/test/bufconn"
)

type testConfig struct {
	DBSource             string        `mapstructure:"DB_SOURCE"`
	TokenKey             string        `mapstructure:"TOKEN_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
}

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
	)

	srv := NewServer(svc, mw)
	lis := grpc.NewServer()
	pb.RegisterUsersServer(lis, srv)
	lis.Serve(bufListener)
}

func TestIntegration_Register(t *testing.T) {
	client := dial(t)
	username := fmt.Sprintf("test_%d", time.Now().UnixNano())
	defer cleanTestUser(t, username)

	res, err := client.Register(context.Background(), &pb.RegisterReq{
		Username: username,
		Password: random.RandomString(10),
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

	// Verify outbox event stored (unpublished) with matching payload
	var outboxCount int
	err = testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM "outbox_events"
		 WHERE aggregate_id = $1::uuid AND event_type = 'userCreated' AND published_at IS NULL`,
		payload.ID,
	).Scan(&outboxCount)
	require.NoError(t, err)
	require.Equal(t, 1, outboxCount)
}

func TestIntegration_RegisterDuplicateUsername(t *testing.T) {
	client := dial(t)
	username, password, _, _ := registerTestUser(t, client)

	_, err := client.Register(context.Background(), &pb.RegisterReq{
		Username: username,
		Password: password,
	})

	requireCode(t, err, codes.AlreadyExists)
}

func TestIntegration_Login(t *testing.T) {
	client := dial(t)
	username, password, _, _ := registerTestUser(t, client)

	accessToken, refreshToken := loginTestUser(t, client, username, password)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)
}

func TestIntegration_LoginWrongPassword(t *testing.T) {
	client := dial(t)
	username, _, _, _ := registerTestUser(t, client)

	_, err := client.Login(context.Background(), &pb.LoginReq{
		Username: username,
		Password: "wrongpassword",
	})

	requireCode(t, err, codes.Unauthenticated)
}

func TestIntegration_Refresh(t *testing.T) {
	client := dial(t)
	_, _, _, refreshToken := registerTestUser(t, client)

	res, err := client.Refresh(context.Background(), &pb.RefreshReq{
		RefreshToken: refreshToken,
	})

	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)

	// Old refresh token is one-time use - second refresh must fail
	_, err = client.Refresh(context.Background(), &pb.RefreshReq{
		RefreshToken: refreshToken,
	})
	requireCode(t, err, codes.Unauthenticated)
}

func TestIntegration_RefreshInvalidToken(t *testing.T) {
	client := dial(t)

	_, err := client.Refresh(context.Background(), &pb.RefreshReq{
		RefreshToken: "invalid-refresh-token",
	})

	requireCode(t, err, codes.Unauthenticated)
}

func TestIntegration_GetUserById(t *testing.T) {
	client := dial(t)
	username, _, _, _ := registerTestUser(t, client)
	_, _, adminToken := createTestAdmin(t, client)

	var userID string
	err := testPool.QueryRow(context.Background(),
		`SELECT id::text FROM users WHERE username = $1`, username,
	).Scan(&userID)
	require.NoError(t, err)

	res, err := client.GetUserById(authCtx(adminToken), &pb.IdReq{Id: userID})

	require.NoError(t, err)
	require.Equal(t, userID, res.User.Id)
	require.Equal(t, username, res.User.Username)
}

func TestIntegration_GetUserByIdNotAdminFails(t *testing.T) {
	client := dial(t)
	_, _, _, accessToken := registerTestUser(t, client)
	username, _, _, _ := registerTestUser(t, client)

	var userID string
	err := testPool.QueryRow(context.Background(),
		`SELECT id::text FROM users WHERE username = $1`, username,
	).Scan(&userID)
	require.NoError(t, err)

	_, err = client.GetUserById(authCtx(accessToken), &pb.IdReq{Id: userID})

	requireCode(t, err, codes.Unauthenticated)
}

func TestIntegration_GetUserByIdNotFound(t *testing.T) {
	client := dial(t)
	_, _, adminToken := createTestAdmin(t, client)

	_, err := client.GetUserById(authCtx(adminToken), &pb.IdReq{Id: "00000000-0000-0000-0000-000000000000"})

	requireCode(t, err, codes.Unauthenticated)
}

func TestIntegration_ListUsers(t *testing.T) {
	client := dial(t)
	registerTestUser(t, client)
	_, _, adminToken := createTestAdmin(t, client)

	res, err := client.ListUsers(authCtx(adminToken), &pb.Empty{})

	require.NoError(t, err)
	require.NotEmpty(t, res.Users)
}

func TestIntegration_ListUsersAsUserFails(t *testing.T) {
	client := dial(t)
	_, _, _, accessToken := registerTestUser(t, client)

	_, err := client.ListUsers(authCtx(accessToken), &pb.Empty{})

	requireCode(t, err, codes.Unauthenticated)
}

func TestIntegration_UpdateUser(t *testing.T) {
	client := dial(t)
	username, _, _, accessToken := registerTestUser(t, client)

	var userID string
	err := testPool.QueryRow(context.Background(),
		`SELECT id::text FROM users WHERE username = $1`, username,
	).Scan(&userID)
	require.NoError(t, err)

	newUsername := fmt.Sprintf("updated_%d", time.Now().UnixNano())
	res, err := client.UpdateUser(authCtx(accessToken), &pb.UpdateUserReq{
		Username: newUsername,
	})

	require.NoError(t, err)
	require.Equal(t, userID, res.User.Id)
	require.Equal(t, newUsername, res.User.Username)
}
