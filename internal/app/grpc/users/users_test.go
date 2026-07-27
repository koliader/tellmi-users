package users_server

import (
	"context"
	"testing"

	"github.com/koliader/tellmi-users/internal/lib/random"
	"github.com/koliader/tellmi-users/internal/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockUserService struct {
	registerFn    func(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error)
	loginFn       func(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error)
	refreshFn     func(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error)
	getUserByIdFn func(ctx context.Context, req *pb.IdReq) (*db.User, error)
	listUsersFn   func(ctx context.Context) (*[]db.User, error)
	updateUserFn  func(ctx context.Context, req *pb.UpdateUserReq) (*db.User, error)
}

func (m *mockUserService) Register(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
	return m.registerFn(ctx, req)
}
func (m *mockUserService) Login(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
	return m.loginFn(ctx, req)
}
func (m *mockUserService) Refresh(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error) {
	return m.refreshFn(ctx, req)
}
func (m *mockUserService) GetUserById(ctx context.Context, req *pb.IdReq) (*db.User, error) {
	return m.getUserByIdFn(ctx, req)
}
func (m *mockUserService) ListUsers(ctx context.Context) (*[]db.User, error) {
	return m.listUsersFn(ctx)
}
func (m *mockUserService) UpdateUser(ctx context.Context, req *pb.UpdateUserReq) (*db.User, error) {
	return m.updateUserFn(ctx, req)
}

func TestGetUserByIdSuccess(t *testing.T) {
	mock := &mockUserService{
		getUserByIdFn: func(ctx context.Context, req *pb.IdReq) (*db.User, error) {
			return &db.User{
				ID:       1,
				Username: "alice",
				Role:     "USER",
			}, nil
		},
	}

	server := NewServer(mock)
	res, err := server.GetUserById(context.Background(), &pb.IdReq{Id: 1})

	require.NoError(t, err)
	require.Equal(t, int64(1), res.User.Id)
	require.Equal(t, "alice", res.User.Username)
	require.Equal(t, "USER", res.User.Role)
}

func TestGetUserByIdNotFound(t *testing.T) {
	mock := &mockUserService{
		getUserByIdFn: func(ctx context.Context, req *pb.IdReq) (*db.User, error) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		},
	}

	server := NewServer(mock)
	_, err := server.GetUserById(context.Background(), &pb.IdReq{Id: 999})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestRegister(t *testing.T) {
	mock := &mockUserService{
		registerFn: func(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
			return &pb.AuthRes{AccessToken: "AccessToken", RefreshToken: "RefreshToken"}, nil
		},
	}

	server := NewServer(mock)
	res, err := server.Register(context.Background(), &pb.RegisterReq{Username: random.RandomString(10), Password: random.RandomString(10)})
	require.NoError(t, err)
	require.Equal(t, "AccessToken", res.AccessToken)
	require.Equal(t, "RefreshToken", res.RefreshToken)
}

func TestRegisterDuplicateUser(t *testing.T) {
	mock := &mockUserService{
		registerFn: func(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
			return nil, status.Errorf(codes.AlreadyExists, "user with this username already exists")
		},
	}

	server := NewServer(mock)
	_, err := server.Register(context.Background(), &pb.RegisterReq{Username: "alice", Password: "pass123"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.AlreadyExists, st.Code())
}

func TestLogin(t *testing.T) {
	mock := &mockUserService{
		loginFn: func(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
			return &pb.AuthRes{AccessToken: "AccessToken", RefreshToken: "RefreshToken"}, nil
		},
	}

	server := NewServer(mock)
	res, err := server.Login(context.Background(), &pb.LoginReq{Username: random.RandomString(10), Password: random.RandomString(10)})
	require.NoError(t, err)
	require.Equal(t, "AccessToken", res.AccessToken)
	require.Equal(t, "RefreshToken", res.RefreshToken)
}

func TestLoginInvalidLoginData(t *testing.T) {
	mock := &mockUserService{loginFn: func(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid login or password")
	}}

	server := NewServer(mock)
	_, err := server.Login(context.Background(), &pb.LoginReq{Username: random.RandomString(10), Password: random.RandomString(10)})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "invalid login or password", st.Message())
}

func TestListUsers(t *testing.T) {
	mock := &mockUserService{
		listUsersFn: func(ctx context.Context) (*[]db.User, error) {
			return &[]db.User{{
				ID:       1,
				Username: "alice",
				Role:     "USER",
			}}, nil
		},
	}

	server := NewServer(mock)
	users, err := server.ListUsers(context.Background(), &pb.Empty{})
	require.NoError(t, err)
	require.NotEmpty(t, users)
}

func TestUpdateUser(t *testing.T) {
	mock := &mockUserService{
		updateUserFn: func(ctx context.Context, req *pb.UpdateUserReq) (*db.User, error) {
			return &db.User{ID: 1, Username: "bob", Role: "User"}, nil
		},
	}

	server := NewServer(mock)
	user, err := server.UpdateUser(context.Background(), &pb.UpdateUserReq{Id: 1, Username: "bob"})
	require.NoError(t, err)
	require.Equal(t, "bob", user.User.Username)
	require.Equal(t, int64(1), user.User.Id)
}
