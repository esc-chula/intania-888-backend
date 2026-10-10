package user

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/pkg/config"
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
	router.Patch("/me", h.UpdateOwnProfile)
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
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		NickName:  user.NickName,
		RoleID:    user.RoleID,
		GroupID:   user.GroupID,
		CreatedAt: user.CreatedAt,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(httpidentity.Response(created))
}

// GetUser serves one account profile.
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

// UpdateOwnProfile applies a partial update to the signed-in user's profile.
func (h *HTTPHandler) UpdateOwnProfile(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var request UpdateOwnProfileRequest
	if err := apierror.BindJSON(c, &request); err != nil {
		return err
	}

	user, err := h.service.UpdateOwnProfile(c.UserContext(), profile.ID, ProfilePatch{
		Name:        request.Name,
		NickName:    request.NickName,
		NickNameSet: request.nickNameSet,
	})
	if err != nil {
		return mapUserError(err)
	}

	return c.JSON(httpidentity.Response(user))
}

// UpdateUser checks the legacy path ID and delegates to UpdateOwnProfile.
//
// Deprecated: use PATCH /users/me through UpdateOwnProfile.
func (h *HTTPHandler) UpdateUser(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	if c.Params("id") != profile.ID {
		return apierror.New(fiber.StatusForbidden, "FORBIDDEN", "You can only update your own profile")
	}

	return h.UpdateOwnProfile(c)
}

// AdminUpdateUser updates administrator-editable account fields.
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
		Name:          userDto.Name,
		NickName:      userDto.NickName,
		GroupID:       userDto.GroupID,
		GroupIDSet:    userDto.groupIDSet,
		RemainingCoin: userDto.RemainingCoin,
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

// RegisterExternalRoutes declares the delegated permission required to spend coins.
func (h *HTTPHandler) RegisterExternalRoutes(router fiber.Router, requireScope func(string) fiber.Handler) {
	router.Post("/deduct-coin", requireScope(config.ScopeCoinsSpend), h.DeductCoin)
}

// DeductCoin deducts coins for an authenticated external client.
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
