package policy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
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
	Kind          string     `json:"kind" validate:"required,oneof=allowlist blacklist"`
	PrincipalType string     `json:"principal_type" validate:"required,oneof=email google_subject"`
	Principal     string     `json:"principal" validate:"required"`
	Reason        string     `json:"reason" validate:"required"`
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
	for field := range fields {
		if field != "reason" && field != "enabled" && field != "expires_at" {
			return errors.New("unknown field")
		}
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

func (r CreatePolicyRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.Kind) == "" {
		details["kind"] = "is required"
	}
	if strings.TrimSpace(r.PrincipalType) == "" {
		details["principal_type"] = "is required"
	}
	if strings.TrimSpace(r.Principal) == "" {
		details["principal"] = "is required"
	}
	if strings.TrimSpace(r.Reason) == "" {
		details["reason"] = "is required"
	}
	return details
}

func (r UpdatePolicyRequest) ValidateRequest() map[string]string {
	if r.Reason == nil && !r.expiresAtSet && r.Enabled == nil {
		return map[string]string{"body": "must include at least one policy field"}
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
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /auth/policies [get]
func (h *HttpHandler) List(c *fiber.Ctx) error {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return apierror.Invalid(map[string]string{"limit": "must be between 1 and 200"})
	}
	offset, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return apierror.Invalid(map[string]string{"cursor": "is invalid"})
	}

	result, err := h.service.List(ListFilter{
		Kind:          strings.TrimSpace(c.Query("kind")),
		PrincipalType: strings.TrimSpace(c.Query("principal_type")),
		Status:        strings.TrimSpace(c.Query("status", StatusActive)),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return mapPolicyError(err)
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
	if err := apierror.BindJSON(c, &request); err != nil {
		return err
	}
	created, err := h.service.Create(CreateInput(request))
	if err != nil {
		return mapPolicyError(err)
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
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /auth/policies [post]
func (h *HttpHandler) Update(c *fiber.Ctx) error {
	var request UpdatePolicyRequest
	if err := apierror.BindJSON(c, &request); err != nil {
		return err
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

	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	updated, err := h.service.Update(c.Params("id"), input)
	if err != nil {
		return mapPolicyError(err)
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
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /auth/policies/{id} [patch]
func (h *HttpHandler) Delete(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	if _, err := h.service.Disable(c.Params("id")); err != nil {
		return mapPolicyError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Disable access policy
// @Description Disables an allowlist or blacklist entry.
// @Tags Auth Policy
// @Param id path string true "policy ID"
// @Success 204 "policy disabled"
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
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

func mapPolicyError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidPolicy):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid access policy")
	case errors.Is(err, ErrPolicyNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Access policy not found")
	case errors.Is(err, ErrPolicyConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "CONFLICT", "Access policy already exists")
	case errors.Is(err, security.ErrPolicyUnavailable):
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Access policy is unavailable")
	default:
		return err
	}
}
