package users_service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	pb "github.com/koliader/tellmi-sdk/proto/pb"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	"github.com/koliader/tellmi-sdk/token"
	"github.com/koliader/tellmi-users/internal/lib/password"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
)

// --- mock Store ---

type mockStore struct {
	createUserFn         func(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	getUserByUsernameFn  func(ctx context.Context, username string) (db.User, error)
	createRefreshTokenFn func(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error)
	getRefreshTokenFn    func(ctx context.Context, token string) (db.RefreshToken, error)
	deleteRefreshTokenFn func(ctx context.Context, token string) error
	getUserByIdFn        func(ctx context.Context, id uuid.UUID) (db.User, error)
	listUsersFn          func(ctx context.Context) ([]db.User, error)
	updateUserFn         func(ctx context.Context, arg db.UpdateUserParams) (db.User, error)
}

func (m *mockStore) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	return m.createUserFn(ctx, arg)
}
func (m *mockStore) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	return m.getUserByUsernameFn(ctx, username)
}
func (m *mockStore) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	return m.createRefreshTokenFn(ctx, arg)
}
func (m *mockStore) GetRefreshToken(ctx context.Context, token string) (db.RefreshToken, error) {
	return m.getRefreshTokenFn(ctx, token)
}
func (m *mockStore) DeleteRefreshToken(ctx context.Context, token string) error {
	return m.deleteRefreshTokenFn(ctx, token)
}
func (m *mockStore) GetUserById(ctx context.Context, id uuid.UUID) (db.User, error) {
	return m.getUserByIdFn(ctx, id)
}
func (m *mockStore) ListUsers(ctx context.Context) ([]db.User, error) {
	return m.listUsersFn(ctx)
}
func (m *mockStore) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.User, error) {
	return m.updateUserFn(ctx, arg)
}
func (m *mockStore) DeleteRefreshTokensByUsername(ctx context.Context, username string) error {
	return nil
}

// --- mock MessageSender ---

type mockSender struct {
	sendMessageFn func(queueName string, message []byte) error
}

func (m *mockSender) SendMessage(queueName string, message []byte) error {
	return m.sendMessageFn(queueName, message)
}

// --- helpers ---

func newTestService(t *testing.T, store db.Store, sender rabbitmq.MessageSender) *Service {
	t.Helper()
	key := "abcdefghijklmnopqrstuvwxyz123456"
	maker, err := token.NewJWTMaker(key)
	require.NoError(t, err)

	svc, err := NewService(maker, time.Minute, time.Hour, store, sender)
	require.NoError(t, err)
	return svc
}

// --- tests ---

func TestServiceRegisterSuccess(t *testing.T) {
	store := &mockStore{
		createUserFn: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{ID: uuid.New(), Username: arg.Username, Password: arg.Password, Role: "USER"}, nil
		},
		createRefreshTokenFn: func(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
			return db.RefreshToken{Token: arg.Token, Username: arg.Username, ExpiresAt: arg.ExpiresAt}, nil
		},
	}
	sender := &mockSender{
		sendMessageFn: func(queueName string, message []byte) error { return nil },
	}

	svc := newTestService(t, store, sender)

	res, err := svc.Register(context.Background(), &pb.RegisterReq{
		Username: "alice",
		Password: "pass123",
	})

	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)
}

func TestServiceRegisterDuplicateUser(t *testing.T) {
	store := &mockStore{
		createUserFn: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{}, fmt.Errorf("duplicate username")
		},
	}
	sender := &mockSender{
		sendMessageFn: func(queueName string, message []byte) error { return nil },
	}

	svc := newTestService(t, store, sender)

	_, err := svc.Register(context.Background(), &pb.RegisterReq{
		Username: "alice",
		Password: "pass123",
	})

	require.Error(t, err)
}

func TestServiceLoginSuccess(t *testing.T) {
	hashedPw, _ := password.HashPassword("pass123")

	store := &mockStore{
		getUserByUsernameFn: func(ctx context.Context, username string) (db.User, error) {
			return db.User{ID: uuid.New(), Username: "alice", Password: hashedPw, Role: "USER"}, nil
		},
		createRefreshTokenFn: func(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
			return db.RefreshToken{Token: arg.Token, Username: arg.Username}, nil
		},
	}
	sender := &mockSender{
		sendMessageFn: func(queueName string, message []byte) error { return nil },
	}

	svc := newTestService(t, store, sender)

	res, err := svc.Login(context.Background(), &pb.LoginReq{
		Username: "alice",
		Password: "pass123",
	})

	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)
}

func TestServiceLoginWrongPassword(t *testing.T) {
	hashedPw, _ := password.HashPassword("correct-password")

	store := &mockStore{
		getUserByUsernameFn: func(ctx context.Context, username string) (db.User, error) {
			return db.User{ID: uuid.New(), Username: "alice", Password: hashedPw, Role: "USER"}, nil
		},
	}
	sender := &mockSender{
		sendMessageFn: func(queueName string, message []byte) error { return nil },
	}

	svc := newTestService(t, store, sender)

	_, err := svc.Login(context.Background(), &pb.LoginReq{
		Username: "alice",
		Password: "wrong-password",
	})

	require.Error(t, err)
}

func TestServiceLoginUserNotFound(t *testing.T) {
	store := &mockStore{
		getUserByUsernameFn: func(ctx context.Context, username string) (db.User, error) {
			return db.User{}, fmt.Errorf("no rows")
		},
	}
	sender := &mockSender{
		sendMessageFn: func(queueName string, message []byte) error { return nil },
	}

	svc := newTestService(t, store, sender)

	_, err := svc.Login(context.Background(), &pb.LoginReq{
		Username: "nobody",
		Password: "pass123",
	})

	require.Error(t, err)
}
