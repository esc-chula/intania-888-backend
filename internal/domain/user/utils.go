package user

import "github.com/wiraphatys/intania888/internal/model"

func ToUserEntity(userDto *model.UserDto) *model.User {
	return &model.User{
		Id:            userDto.Id,
		Email:         userDto.Email,
		Name:          userDto.Name,
		RoleId:        userDto.RoleId,
		RemainingCoin: userDto.RemainingCoin,
	}
}
