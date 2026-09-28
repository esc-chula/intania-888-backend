package user

import (
	"errors"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
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

	// Admin routes
	adminRouter := router.Group("/admin", mid.AdminMiddleware)
	adminRouter.Patch("/:id", h.AdminUpdateUser)
}

func (h *UserHttpHandler) CreateUser(c *fiber.Ctx) error {
	user := new(model.UserDto)

	if err := apierror.BindJSON(c, user); err != nil {
		return err
	}

	if err := h.service.CreateUser(user); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

// @Summary Get user by ID
// @Description Retrieves a single user by their ID
// @Tags User
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Success 200    {object} model.UserDto
// @Failure 404    {object} apierror.Response  "user not found"
// @Router  /users/{id} [get]
// @Security BearerAuth
func (h *UserHttpHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if strings.TrimSpace(id) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	user, err := h.service.GetUser(id)
	if err != nil {
		return mapUserError(err)
	}

	return c.JSON(user)
}

// @Summary Get all users
// @Description Retrieves a list of all users
// @Tags User
// @Produce  json
// @Success 200    {array}  model.UserDto
// @Failure 500    {object} apierror.Response  "internal server error"
// @Router  /users [get]
// @Security BearerAuth
func (h *UserHttpHandler) GetAllUsers(c *fiber.Ctx) error {
	users, err := h.service.GetAllUsers()
	if err != nil {
		return err
	}

	return c.JSON(users)
}

// @Summary Update user
// @Description Updates an existing user
// @Tags User
// @Accept  json
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Param   user  body      model.UpdateUserDto  true  "Updated user information"
// @Success 200    {object} model.UserDto
// @Failure 400    {object} apierror.Response  "cannot parse body"
// @Failure 401    {object} apierror.Response  "unauthorized"
// @Failure 404    {object} apierror.Response  "user not found"
// @Failure 500    {object} apierror.Response  "internal server error"
// @Router  /users/{id} [patch]
// @Security BearerAuth
func (h *UserHttpHandler) UpdateUser(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)

	updateUserDto := new(model.UpdateUserDto)
	if err := apierror.BindJSON(c, updateUserDto); err != nil {
		return err
	}
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	user := model.UserDto{
		Id:            profile.Id,
		Email:         profile.Email,
		Name:          updateUserDto.Name,
		NickName:      updateUserDto.NickName,
		RoleId:        profile.RoleId,
		GroupId:       updateUserDto.GroupId,
		RemainingCoin: model.Money{},
	}

	if err := h.service.UpdateUser(&user); err != nil {
		return mapUserError(err)
	}

	return c.JSON(user)
}

// @Summary Admin update user
// @Description Allows admin to update user profile and coins. Role changes use the operator database workflow.
// @Tags User
// @Accept  json
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Param   user  body      model.AdminUpdateUserDto  true  "Updated user information"
// @Success 200    {object} model.UserDto
// @Failure 400    {object} apierror.Response  "cannot parse body"
// @Failure 401    {object} apierror.Response  "unauthorized"
// @Failure 403    {object} apierror.Response  "administrator permission required"
// @Failure 404    {object} apierror.Response  "user not found"
// @Failure 500    {object} apierror.Response  "internal server error"
// @Router  /users/admin/{id} [patch]
// @Security BearerAuth
func (h *UserHttpHandler) AdminUpdateUser(c *fiber.Ctx) error {
	userId := c.Params("id")
	if strings.TrimSpace(userId) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	userDto := new(model.AdminUpdateUserDto)
	if err := apierror.BindJSON(c, userDto); err != nil {
		return err
	}

	if err := h.service.AdminUpdateUser(userId, userDto); err != nil {
		return mapUserError(err)
	}

	// Return updated user
	updatedUser, err := h.service.GetUser(userId)
	if err != nil {
		return mapUserError(err)
	}

	return c.JSON(updatedUser)
}

// Deprecated: available until the original integration is understood.
// RegisterExternalRoutes registers the deprecated external coin API. Keep it
// available until the original integration and its consumers are understood.
func (h *UserHttpHandler) RegisterExternalRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router.Post("/deduct-coin", mid.ExternalAPIMiddleware, h.DeductCoin)
}

// @Summary Deduct coins from user balance (External API)
// @Description External API endpoint to deduct coins from authenticated user's balance. Bypasses browser-only validation but requires JWT authentication.
// @Tags External
// @Deprecated
// @Accept json
// @Produce json
// @Param request body model.DeductCoinRequest true "Deduction request"
// @Success 200 {object} model.DeductCoinResponse
// @Failure 400 {object} apierror.Response "Invalid amount or parse error"
// @Failure 401 {object} apierror.Response "Missing or invalid token"
// @Failure 404 {object} apierror.Response "User not found"
// @Failure 422 {object} apierror.Response "Insufficient balance"
// @Failure 500 {object} apierror.Response "Internal server error"
// @Router /external/deduct-coin [post]
// @Security BearerAuth
func (h *UserHttpHandler) DeductCoin(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req model.DeductCoinRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	// Call service to deduct coins
	remainingBalance, err := h.service.DeductCoin(profile.Id, req.Amount)
	if err != nil {
		return mapUserError(err)
	}

	// Return success response
	return c.Status(fiber.StatusOK).JSON(model.DeductCoinResponse{
		Success:          true,
		DeductedAmount:   req.Amount,
		RemainingBalance: remainingBalance,
	})
}

func mapUserError(err error) error {
	switch {
	case errors.Is(err, ErrUserNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "User not found")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	default:
		return err
	}
}
