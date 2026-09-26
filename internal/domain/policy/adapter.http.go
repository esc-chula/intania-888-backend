package policy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/gofiber/fiber/v2"
)

type HttpHandler struct {
	service Service
}

func NewHttpHandler(service Service) *HttpHandler {
	return &HttpHandler{service: service}
}

func (h *HttpHandler) RegisterRoutes(router fiber.Router, authMiddleware, adminMiddleware fiber.Handler) {
	protected := router.Group("/auth", authMiddleware).Group("/policies", adminMiddleware)
	protected.Get("/", h.List)
	protected.Post("/", h.Create)
	protected.Patch("/:id", h.Update)
	protected.Delete("/:id", h.Delete)
}

type CreatePolicyRequest struct {
	Kind          string     `json:"kind"`
	PrincipalType string     `json:"principal_type"`
	Principal     string     `json:"principal"`
	Reason        string     `json:"reason"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

type UpdatePolicyRequest struct {
	Reason       *string    `json:"reason"`
	ExpiresAt    *time.Time `json:"expires_at"`
	Enabled      *bool      `json:"enabled"`
	expiresAtSet bool
}

func (r *UpdatePolicyRequest) UnmarshalJSON(data []byte) error {
	type wire struct {
		Reason    *string `json:"reason"`
		Enabled   *bool   `json:"enabled"`
		ExpiresAt string  `json:"expires_at"`
	}
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	r.Reason = value.Reason
	r.Enabled = value.Enabled
	r.ExpiresAt = nil
	r.expiresAtSet = false
	if raw, ok := fields["expires_at"]; ok {
		r.expiresAtSet = true
		if string(raw) != "null" {
			var expiresAt time.Time
			if err := json.Unmarshal(raw, &expiresAt); err != nil {
				return err
			}
			r.ExpiresAt = &expiresAt
		}
	}
	return nil
}

type PolicyListResponse struct {
	Items      []*AccessPolicy `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

// @Summary List access policies
// @Description Lists allowlist and blacklist entries for an administrator.
// @Tags Auth Policy
// @Produce json
// @Param kind query string false "allowlist or blacklist"
// @Param principal_type query string false "email or google_subject"
// @Param status query string false "active, inactive, or all"
// @Param limit query int false "page size, maximum 200"
// @Param cursor query string false "opaque pagination cursor"
// @Success 200 {object} PolicyListResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /auth/policies [get]
func (h *HttpHandler) List(c *fiber.Ctx) error {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return policyError(c, fiber.StatusBadRequest, "invalid limit")
	}
	offset, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return policyError(c, fiber.StatusBadRequest, "invalid cursor")
	}

	result, err := h.service.List(ListFilter{
		Kind:          strings.TrimSpace(c.Query("kind")),
		PrincipalType: strings.TrimSpace(c.Query("principal_type")),
		Status:        strings.TrimSpace(c.Query("status", StatusActive)),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return mapPolicyError(c, err)
	}

	response := PolicyListResponse{Items: result.Items}
	if result.HasMore {
		next := encodeCursor(offset + len(result.Items))
		response.NextCursor = &next
	}
	return c.JSON(response)
}

func (h *HttpHandler) Create(c *fiber.Ctx) error {
	var request CreatePolicyRequest
	if err := c.BodyParser(&request); err != nil {
		return policyError(c, fiber.StatusBadRequest, "invalid policy request")
	}
	created, err := h.service.Create(CreateInput(request))
	if err != nil {
		return mapPolicyError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

// @Summary Create access policy
// @Description Creates an allowlist or blacklist entry.
// @Tags Auth Policy
// @Accept json
// @Produce json
// @Param policy body CreatePolicyRequest true "access policy"
// @Success 201 {object} AccessPolicy
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /auth/policies [post]
func (h *HttpHandler) Update(c *fiber.Ctx) error {
	var request UpdatePolicyRequest
	if err := c.BodyParser(&request); err != nil {
		return policyError(c, fiber.StatusBadRequest, "invalid policy request")
	}

	input := UpdateInput{}
	if request.Reason != nil {
		input.ReasonSet = true
		input.Reason = *request.Reason
	}
	if request.expiresAtSet {
		input.ExpiresAtSet = true
		input.ExpiresAt = request.ExpiresAt
	}
	if request.Enabled != nil {
		input.EnabledSet = true
		input.Enabled = *request.Enabled
	}

	updated, err := h.service.Update(c.Params("id"), input)
	if err != nil {
		return mapPolicyError(c, err)
	}
	return c.JSON(updated)
}

// @Summary Update access policy
// @Description Updates reason, expiry, or enabled state. Identity and kind are immutable.
// @Tags Auth Policy
// @Accept json
// @Produce json
// @Param id path string true "policy ID"
// @Param policy body UpdatePolicyRequest true "policy changes"
// @Success 200 {object} AccessPolicy
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /auth/policies/{id} [patch]
func (h *HttpHandler) Delete(c *fiber.Ctx) error {
	if _, err := h.service.Disable(c.Params("id")); err != nil {
		return mapPolicyError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Disable access policy
// @Description Disables an allowlist or blacklist entry.
// @Tags Auth Policy
// @Param id path string true "policy ID"
// @Success 204 "policy disabled"
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /auth/policies/{id} [delete]
func parseLimit(raw string) (int, error) {
	if raw == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 200 {
		return 0, errors.New("invalid limit")
	}
	return limit, nil
}

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return offset, nil
}

func policyError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

func mapPolicyError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrInvalidPolicy):
		return policyError(c, http.StatusBadRequest, "invalid access policy")
	case errors.Is(err, ErrPolicyNotFound):
		return policyError(c, http.StatusNotFound, "access policy not found")
	case errors.Is(err, ErrPolicyConflict):
		return policyError(c, http.StatusConflict, "access policy already exists")
	case errors.Is(err, security.ErrPolicyUnavailable):
		return policyError(c, http.StatusServiceUnavailable, "access policy is unavailable")
	default:
		return policyError(c, http.StatusInternalServerError, "unable to update access policy")
	}
}
