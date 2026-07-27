package converter

import (
	"github.com/koliader/tellmi-sdk/proto/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

func ConvertUser(user db.User) *pb.User {
	return &pb.User{
		Id:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		IsBlocked: user.IsBlocked,
	}
}

func ConvertUsers(users []db.User) []*pb.User {
	converted := make([]*pb.User, 0, len(users))
	for _, user := range users {
		converted = append(converted, ConvertUser(user))
	}
	return converted
}
