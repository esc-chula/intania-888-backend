package user

import (
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
)

type UserHttpHandler struct {
	service UserService
}

func NewUserHttpHandler(service UserService) *UserHttpHandler {
	return &UserHttpHandler{service: service}
}

func (h *UserHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/users", mid.AuthMiddleware)

	router.Get("/", h.GetAllUsers)
	router.Get("/:id", h.GetUser)
	router.Patch("/:id", h.UpdateUser)
}

// @Summary Create a new user
// @Description Creates a new user and stores it in the system
// @Tags User
// @Accept  json
// @Produce  json
// @Param   user  body      model.UserDto  true  "User information"
// @Success 201    {object} model.UserDto
// @Failure 400    {object} map[string]string  "cannot parse body"
// @Failure 500    {object} map[string]string  "internal server error"
// @Router  /users [post]
func (h *UserHttpHandler) CreateUser(c *fiber.Ctx) error {
	user := new(model.UserDto)
	if err := c.BodyParser(user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse body"})
	}
	if err := h.service.CreateUser(user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}

// @Summary Get user by ID
// @Description Retrieves a single user by their ID
// @Tags User
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Success 200    {object} model.UserDto
// @Failure 404    {object} map[string]string  "user not found"
// @Router  /users/{id} [get]
func (h *UserHttpHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.service.GetUser(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
}

// @Summary Get all users
// @Description Retrieves a list of all users
// @Tags User
// @Produce  json
// @Success 200    {array}  model.UserDto
// @Failure 500    {object} map[string]string  "internal server error"
// @Router  /users [get]
func (h *UserHttpHandler) GetAllUsers(c *fiber.Ctx) error {
	users, err := h.service.GetAllUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(users)
}

// @Summary Update user
// @Description Updates an existing user
// @Tags User
// @Accept  json
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Param   user  body      model.UserDto  true  "Updated user information"
// @Success 200    {object} model.UserDto
// @Failure 400    {object} map[string]string  "cannot parse body"
// @Failure 500    {object} map[string]string  "internal server error"
// @Router  /users/{id} [patch]
func (h *UserHttpHandler) UpdateUser(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)

	updateUserDto := new(model.UpdateUserDto)
	if err := c.BodyParser(updateUserDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse body"})
	}
	user := model.UserDto{
		Id:            profile.Id,
		Email:         profile.Email,
		Name:          updateUserDto.Name,
		NickName:      updateUserDto.NickName,
		RoleId:        profile.RoleId,
		GroupId:       updateUserDto.GroupId,
		RemainingCoin: 0.00,
	}
	if err := h.service.UpdateUser(&user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(user)
}
