package converter

import (
	pb "github.com/koliader/tellmi-users/internal/pb"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
)

func ConvertUser(user db.User) *pb.User {
	u := pb.User{
		Id:       user.ID,
		Username: user.Username,
	}

	return &u
}

func ConvertUsers(users []db.User) []*pb.User {
	converted := make([]*pb.User, 0, len(users))
	for _, user := range users {
		u := pb.User{
			Id:        user.ID,
			Username:  user.Username,
			Role:      user.Role,
			IsBlocked: user.IsBlocked,
		}
		converted = append(converted, &u)
	}
	return converted
}
