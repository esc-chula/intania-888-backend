package user

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
)

// HTTPHandler serves account HTTP endpoints.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs account HTTP handlers.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes installs browser account routes using the supplied authorization middleware.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, auth, admin fiber.Handler) {
	router = router.Group("/users", auth)

	router.Get("/", h.GetAllUsers)
	router.Get("/:id", h.GetUser)
	router.Patch("/:id", h.UpdateUser)

	// Admin routes
	adminRouter := router.Group("/admin", admin)
	adminRouter.Patch("/:id", h.AdminUpdateUser)
}

// CreateUser parses and creates a user account.
func (h *HTTPHandler) CreateUser(c *fiber.Ctx) error {
	user := new(CreateUserRequest)

	if err := apierror.BindJSON(c, user); err != nil {
		return err
	}

	created, err := h.service.CreateUser(c.UserContext(), CreateInput{
		ID: user.ID, Email: user.Email, Name: user.Name, NickName: user.NickName,
		RoleID: user.RoleID, GroupID: user.GroupID, CreatedAt: user.CreatedAt,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(httpidentity.Response(created))
}

// GetUser serves one account profile.
//
// @Summary Get user by ID
// @Description Retrieves a single user by their ID
// @Tags User
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Success 200    {object} httpidentity.ProfileResponse
// @Failure 404    {object} apierror.Response  "user not found"
// @Router  /users/{id} [get]
// @Security CookieSession
func (h *HTTPHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if strings.TrimSpace(id) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	user, err := h.service.GetUser(c.UserContext(), id)
	if err != nil {
		return mapUserError(err)
	}

	return c.JSON(httpidentity.Response(user))
}

// GetAllUsers serves every account profile.
//
// @Summary Get all users
// @Description Retrieves a list of all users
// @Tags User
// @Produce  json
// @Success 200    {array}  httpidentity.ProfileResponse
// @Failure 500    {object} apierror.Response  "internal server error"
// @Router  /users [get]
// @Security CookieSession
func (h *HTTPHandler) GetAllUsers(c *fiber.Ctx) error {
	users, err := h.service.GetAllUsers(c.UserContext())
	if err != nil {
		return err
	}

	responses := make([]*httpidentity.ProfileResponse, len(users))
	for i, profile := range users {
		responses[i] = httpidentity.Response(profile)
	}
	return c.JSON(responses)
}

// UpdateUser updates the authenticated account rather than the path account, preserving current behavior.
//
// @Summary Update user
// @Description Updates an existing user
// @Tags User
// @Accept  json
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Param   user  body      UpdateUserRequest  true  "Updated user information"
// @Success 200    {object} httpidentity.ProfileResponse
// @Failure 400    {object} apierror.Response  "cannot parse body"
// @Failure 401    {object} apierror.Response  "unauthorized"
// @Failure 404    {object} apierror.Response  "user not found"
// @Failure 500    {object} apierror.Response  "internal server error"
// @Router  /users/{id} [patch]
// @Security CookieSession
func (h *HTTPHandler) UpdateUser(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)

	updateUserDto := new(UpdateUserRequest)
	if err := apierror.BindJSON(c, updateUserDto); err != nil {
		return err
	}
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	user, err := h.service.UpdateUser(c.UserContext(), UpdateInput{
		ID:       profile.ID,
		Email:    profile.Email,
		Name:     updateUserDto.Name,
		NickName: updateUserDto.NickName,
		RoleID:   profile.RoleID,
		GroupID:  updateUserDto.GroupID,
	})
	if err != nil {
		return mapUserError(err)
	}

	return c.JSON(httpidentity.Response(user))
}

// AdminUpdateUser updates administrator-editable account fields.
//
// @Summary Admin update user
// @Description Allows admin to update user profile and coins. Role changes use the operator database workflow.
// @Tags User
// @Accept  json
// @Produce  json
// @Param   id    path      string  true  "User ID"
// @Param   user  body      AdminUpdateUserRequest  true  "Updated user information"
// @Success 200    {object} httpidentity.ProfileResponse
// @Failure 400    {object} apierror.Response  "cannot parse body"
// @Failure 401    {object} apierror.Response  "unauthorized"
// @Failure 403    {object} apierror.Response  "administrator permission required"
// @Failure 404    {object} apierror.Response  "user not found"
// @Failure 500    {object} apierror.Response  "internal server error"
// @Router  /users/admin/{id} [patch]
// @Security CookieSession
func (h *HTTPHandler) AdminUpdateUser(c *fiber.Ctx) error {
	userID := c.Params("id")
	if strings.TrimSpace(userID) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	userDto := new(AdminUpdateUserRequest)
	if err := apierror.BindJSON(c, userDto); err != nil {
		return err
	}

	if err := h.service.AdminUpdateUser(c.UserContext(), userID, AdminUpdateInput{
		Name: userDto.Name, NickName: userDto.NickName,
		GroupID: userDto.GroupID, RemainingCoin: userDto.RemainingCoin,
	}); err != nil {
		return mapUserError(err)
	}

	// Return updated user
	updatedUser, err := h.service.GetUser(c.UserContext(), userID)
	if err != nil {
		return mapUserError(err)
	}

	return c.JSON(httpidentity.Response(updatedUser))
}

// RegisterExternalRoutes registers the deprecated external coin API. Keep it
// available until the original integration and its consumers are understood.
//
// Deprecated: available until the original integration is understood.
func (h *HTTPHandler) RegisterExternalRoutes(router fiber.Router, externalAuth fiber.Handler) {
	router.Post("/deduct-coin", externalAuth, h.DeductCoin)
}

// DeductCoin deducts coins for an authenticated external client.
//
// @Summary Deduct coins from user balance (External API)
// @Description External API endpoint to deduct coins from authenticated user's balance. Bypasses browser-only validation but requires JWT authentication.
// @Tags External
// @Deprecated
// @Accept json
// @Produce json
// @Param request body DeductCoinRequest true "Deduction request"
// @Success 200 {object} DeductCoinResponse
// @Failure 400 {object} apierror.Response "Invalid amount or parse error"
// @Failure 401 {object} apierror.Response "Missing or invalid token"
// @Failure 404 {object} apierror.Response "User not found"
// @Failure 422 {object} apierror.Response "Insufficient balance"
// @Failure 500 {object} apierror.Response "Internal server error"
// @Router /external/deduct-coin [post]
// @Security BearerAuth
func (h *HTTPHandler) DeductCoin(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req DeductCoinRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	// Call service to deduct coins
	remainingBalance, err := h.service.DeductCoin(c.UserContext(), profile.ID, req.Amount)
	if err != nil {
		return mapUserError(err)
	}

	// Return success response
	return c.Status(fiber.StatusOK).JSON(DeductCoinResponse{
		Success:          true,
		DeductedAmount:   req.Amount,
		RemainingBalance: remainingBalance,
	})
}
