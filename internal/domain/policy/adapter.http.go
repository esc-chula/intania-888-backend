package policy

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

// HTTPHandler exposes administrator policy operations.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs the policy HTTP adapter.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes protects all policy routes with authentication and administrator checks.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authMiddleware, adminMiddleware fiber.Handler) {
	protected := router.Group("/auth", authMiddleware).Group("/policies", adminMiddleware)
	protected.Get("/", h.List)
	protected.Post("/", h.Create)
	protected.Patch("/:id", h.Update)
	protected.Delete("/:id", h.Delete)
}

// List returns the filtered policy page and next cursor.
// @Summary List access policies
// @Description Lists allowlist and blacklist entries for an administrator.
// @Tags Auth Policy
// @Produce json
// @Param kind query string false "allowlist or blacklist"
// @Param principal_type query string false "email or google_subject"
// @Param status query string false "active, inactive, or all"
// @Param limit query int false "page size, maximum 200"
// @Param cursor query string false "opaque pagination cursor"
// @Success 200 {object} ListResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /auth/policies [get]
// @Security CookieSession
func (h *HTTPHandler) List(c *fiber.Ctx) error {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return apierror.Invalid(map[string]string{"limit": "must be between 1 and 200"})
	}
	offset, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return apierror.Invalid(map[string]string{"cursor": "is invalid"})
	}

	result, err := h.service.List(c.UserContext(), ListFilter{
		Kind:          strings.TrimSpace(c.Query("kind")),
		PrincipalType: strings.TrimSpace(c.Query("principal_type")),
		Status:        strings.TrimSpace(c.Query("status", StatusActive)),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return mapPolicyError(err)
	}

	response := ListResponse{Items: policiesToResponse(result.Items)}
	if result.HasMore {
		next := encodeCursor(offset + len(result.Items))
		response.NextCursor = &next
	}
	return c.JSON(response)
}

// Create creates a normalized access policy.
// @Summary Create access policy
// @Description Creates an allowlist or blacklist entry.
// @Tags Auth Policy
// @Accept json
// @Produce json
// @Param policy body CreatePolicyRequest true "access policy"
// @Success 201 {object} Response
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Param X-CSRF-Token header string true "Session-bound CSRF token returned by /auth/me"
// @Router /auth/policies [post]
// @Security CookieSession
func (h *HTTPHandler) Create(c *fiber.Ctx) error {
	var request CreatePolicyRequest
	if err := apierror.BindJSON(c, &request); err != nil {
		return err
	}
	created, err := h.service.Create(c.UserContext(), CreateInput(request))
	if err != nil {
		return mapPolicyError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(policyToResponse(created))
}

// Update changes mutable fields while preserving policy identity.
// @Summary Update access policy
// @Description Updates reason, expiry, or enabled state. Identity and kind are immutable.
// @Tags Auth Policy
// @Accept json
// @Produce json
// @Param id path string true "policy ID"
// @Param policy body UpdatePolicyRequest true "policy changes"
// @Success 200 {object} Response
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Param X-CSRF-Token header string true "Session-bound CSRF token returned by /auth/me"
// @Router /auth/policies/{id} [patch]
// @Security CookieSession
func (h *HTTPHandler) Update(c *fiber.Ctx) error {
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
	updated, err := h.service.Update(c.UserContext(), c.Params("id"), input)
	if err != nil {
		return mapPolicyError(err)
	}
	return c.JSON(policyToResponse(updated))
}

// Delete disables a policy without deleting its audit history.
// @Summary Disable access policy
// @Description Disables an allowlist or blacklist entry.
// @Tags Auth Policy
// @Param id path string true "policy ID"
// @Success 204 "policy disabled"
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Param X-CSRF-Token header string true "Session-bound CSRF token returned by /auth/me"
// @Router /auth/policies/{id} [delete]
// @Security CookieSession
func (h *HTTPHandler) Delete(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	if _, err := h.service.Disable(c.UserContext(), c.Params("id")); err != nil {
		return mapPolicyError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

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

// ServicePort is the policy functionality consumed by HTTP administration.
type ServicePort interface {
	// List returns the selected policy page.
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	// Create adds a policy entry.
	Create(ctx context.Context, input CreateInput) (*AccessPolicy, error)
	// Update changes mutable fields of an existing entry.
	Update(ctx context.Context, id string, input UpdateInput) (*AccessPolicy, error)
	// Disable idempotently disables an existing entry.
	Disable(ctx context.Context, id string) (*AccessPolicy, error)
}
