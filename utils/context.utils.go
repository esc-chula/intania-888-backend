package utils

import (
	"github.com/gofiber/fiber/v2"
	"github.com/wiraphatys/intania888/internal/model"
)

func GetUserProfileFromCtx(c *fiber.Ctx) *model.UserDto {
	// get user from context
	userDto, ok := c.Locals("user").(*model.UserDto)
	if !ok {
		return nil
	}
	return userDto
}
