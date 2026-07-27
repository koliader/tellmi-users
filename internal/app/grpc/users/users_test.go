package users_server

import (
	"context"
	"fmt"
	"testing"

	"github.com/koliader/tellmi-sdk/token"
	pb "github.com/koliader/tellmi-sdk/proto/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- mock UserService ---

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

// --- mock Middleware ---

type mockMiddleware struct {
	authorizeUserFn  func(ctx context.Context) (*token.Payload, error)
	authorizeAdminFn func(ctx context.Context) (*token.Payload, error)
}

func (m *mockMiddleware) AuthorizeUser(ctx context.Context) (*token.Payload, error) {
	return m.authorizeUserFn(ctx)
}
func (m *mockMiddleware) AuthorizeAdmin(ctx context.Context) (*token.Payload, error) {
	return m.authorizeAdminFn(ctx)
}

// --- helpers ---

func newAuthorizedMiddleware() *mockMiddleware {
	return &mockMiddleware{
		authorizeUserFn: func(ctx context.Context) (*token.Payload, error) {
			return &token.Payload{Username: "alice", Role: "USER"}, nil
		},
		authorizeAdminFn: func(ctx context.Context) (*token.Payload, error) {
			return &token.Payload{Username: "admin", Role: "ADMIN"}, nil
		},
	}
}

func newDeniedMiddleware() *mockMiddleware {
	return &mockMiddleware{
		authorizeUserFn: func(ctx context.Context) (*token.Payload, error) {
			return nil, fmt.Errorf("unauthorized")
		},
		authorizeAdminFn: func(ctx context.Context) (*token.Payload, error) {
			return nil, fmt.Errorf("unauthorized")
		},
	}
}

// --- Register tests ---

func TestRegister(t *testing.T) {
	mock := &mockUserService{
		registerFn: func(ctx context.Context, req *pb.RegisterReq) (*pb.AuthRes, error) {
			return &pb.AuthRes{AccessToken: "AccessToken", RefreshToken: "RefreshToken"}, nil
		},
	}

	server := NewServer(mock, newAuthorizedMiddleware())
	res, err := server.Register(context.Background(), &pb.RegisterReq{Username: "alice", Password: "pass123"})
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

	server := NewServer(mock, newAuthorizedMiddleware())
	_, err := server.Register(context.Background(), &pb.RegisterReq{Username: "alice", Password: "pass123"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.AlreadyExists, st.Code())
}

// --- Login tests ---

func TestLogin(t *testing.T) {
	mock := &mockUserService{
		loginFn: func(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
			return &pb.AuthRes{AccessToken: "AccessToken", RefreshToken: "RefreshToken"}, nil
		},
	}

	server := NewServer(mock, newAuthorizedMiddleware())
	res, err := server.Login(context.Background(), &pb.LoginReq{Username: "alice", Password: "pass123"})
	require.NoError(t, err)
	require.Equal(t, "AccessToken", res.AccessToken)
	require.Equal(t, "RefreshToken", res.RefreshToken)
}

func TestLoginInvalidLoginData(t *testing.T) {
	mock := &mockUserService{loginFn: func(ctx context.Context, req *pb.LoginReq) (*pb.AuthRes, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid login or password")
	}}

	server := NewServer(mock, newAuthorizedMiddleware())
	_, err := server.Login(context.Background(), &pb.LoginReq{Username: "alice", Password: "pass123"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "invalid login or password", st.Message())
}

// --- Refresh tests ---

func TestRefresh(t *testing.T) {
	mock := &mockUserService{
		refreshFn: func(ctx context.Context, req *pb.RefreshReq) (*pb.RefreshRes, error) {
			return &pb.RefreshRes{AccessToken: "newAccess", RefreshToken: "newRefresh"}, nil
		},
	}

	server := NewServer(mock, newAuthorizedMiddleware())
	res, err := server.Refresh(context.Background(), &pb.RefreshReq{RefreshToken: "oldRefresh"})
	require.NoError(t, err)
	require.Equal(t, "newAccess", res.AccessToken)
	require.Equal(t, "newRefresh", res.RefreshToken)
}

// --- GetUserById tests ---

func TestGetUserByIdSuccess(t *testing.T) {
	mock := &mockUserService{
		getUserByIdFn: func(ctx context.Context, req *pb.IdReq) (*db.User, error) {
			return &db.User{ID: 1, Username: "alice", Role: "USER"}, nil
		},
	}

	server := NewServer(mock, newAuthorizedMiddleware())
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

	server := NewServer(mock, newAuthorizedMiddleware())
	_, err := server.GetUserById(context.Background(), &pb.IdReq{Id: 999})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestGetUserByIdUnauthorized(t *testing.T) {
	mock := &mockUserService{}
	server := NewServer(mock, newDeniedMiddleware())

	_, err := server.GetUserById(context.Background(), &pb.IdReq{Id: 1})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

// --- ListUsers tests ---

func TestListUsersSuccess(t *testing.T) {
	mock := &mockUserService{
		listUsersFn: func(ctx context.Context) (*[]db.User, error) {
			return &[]db.User{{ID: 1, Username: "alice", Role: "USER"}}, nil
		},
	}

	server := NewServer(mock, newAuthorizedMiddleware())
	users, err := server.ListUsers(context.Background(), &pb.Empty{})
	require.NoError(t, err)
	require.NotEmpty(t, users)
}

func TestListUsersUnauthorized(t *testing.T) {
	mock := &mockUserService{}
	server := NewServer(mock, newDeniedMiddleware())

	_, err := server.ListUsers(context.Background(), &pb.Empty{})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

// --- UpdateUser tests ---

func TestUpdateUserSuccess(t *testing.T) {
	mock := &mockUserService{
		updateUserFn: func(ctx context.Context, req *pb.UpdateUserReq) (*db.User, error) {
			return &db.User{ID: 1, Username: "bob", Role: "User"}, nil
		},
	}

	server := NewServer(mock, newAuthorizedMiddleware())
	user, err := server.UpdateUser(context.Background(), &pb.UpdateUserReq{Id: 1, Username: "bob"})
	require.NoError(t, err)
	require.Equal(t, "bob", user.User.Username)
	require.Equal(t, int64(1), user.User.Id)
}

func TestUpdateUserUnauthorized(t *testing.T) {
	mock := &mockUserService{}
	server := NewServer(mock, newDeniedMiddleware())

	_, err := server.UpdateUser(context.Background(), &pb.UpdateUserReq{Id: 1, Username: "bob"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}
