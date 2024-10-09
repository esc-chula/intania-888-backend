package user

import "github.com/esc-chula/intania-888-backend/internal/model"

func ToUserEntity(userDto *model.UserDto) *model.User {
	return &model.User{
		Id:            userDto.Id,
		Email:         userDto.Email,
		Name:          userDto.Name,
		RoleId:        userDto.RoleId,
		GroupId:       userDto.GroupId,
		NickName:      userDto.NickName,
		RemainingCoin: userDto.RemainingCoin,
	}
}
