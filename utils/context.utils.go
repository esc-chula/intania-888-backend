package utils

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

func GetUserProfileFromCtx(c *fiber.Ctx) *model.UserDto {
	// get user from context
	userDto, ok := c.Locals("user").(*model.UserDto)
	if !ok {
		return nil
	}
	return userDto
}
