package bill

import (
	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
)

// HTTPHandler translates HTTP requests into bill application use cases.
type HTTPHandler struct {
	service HTTPService
}

// NewHTTPHandler constructs the bill HTTP adapter.
func NewHTTPHandler(s HTTPService) *HTTPHandler {
	return &HTTPHandler{service: s}
}

// RegisterRoutes registers the existing bill routes and authentication guards.
func (h *HTTPHandler) RegisterRoutes(r fiber.Router, auth, admin fiber.Handler) {
	r = r.Group("/bills", auth)

	r.Post("/", h.CreateBill)
	r.Get("/", h.GetAllBills)
	r.Get("/:id", h.GetBill)

	a := r.Group("/admin", admin)

	a.Get("/all", h.GetAllBillsAdmin)
	a.Put("/:id/void", h.VoidBill)
}

// CreateBill validates a browser stake and creates a bill for the authenticated actor.
func (h *HTTPHandler) CreateBill(c *fiber.Ctx) error {
	u := httpidentity.GetProfile(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req CreateBillRequest

	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	v, err := h.service.CreateBill(c.UserContext(), u.ID, createInput(req))

	if err != nil {
		return mapBillError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(billResultDTO(v))
}

// GetBill returns one bill belonging to the authenticated actor.
func (h *HTTPHandler) GetBill(c *fiber.Ctx) error {
	u := httpidentity.GetProfile(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	if c.Params("id") == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	v, e := h.service.GetBill(c.UserContext(), c.Params("id"), u.ID)

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(billResultDTO(v))
}

// GetAllBills lists the authenticated actor's bills in their existing order.
func (h *HTTPHandler) GetAllBills(c *fiber.Ctx) error {
	u := httpidentity.GetProfile(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	v, e := h.service.GetAllBills(c.UserContext(), u.ID)

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(billResultsDTO(v))
}

// GetAllBillsAdmin lists every bill after the administrator route guard.
func (h *HTTPHandler) GetAllBillsAdmin(c *fiber.Ctx) error {
	v, e := h.service.GetAllBillsAdmin(c.UserContext())

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(billResultsDTO(v))
}

// VoidBill validates an audit reason and refunds a pending bill for an administrator.
func (h *HTTPHandler) VoidBill(c *fiber.Ctx) error {
	u := httpidentity.GetProfile(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	if c.Params("id") == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	var req VoidBillRequest

	if e := apierror.BindJSON(c, &req); e != nil {
		return e
	}

	v, e := h.service.VoidBill(c.UserContext(), c.Params("id"), u.ID, req.Reason)

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(billResultDTO(v))
}
