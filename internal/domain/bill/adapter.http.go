package bill

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BillHttpHandler struct {
	service BillService
}

func NewBillHttpHandler(s BillService) *BillHttpHandler {
	return &BillHttpHandler{service: s}
}

func (h *BillHttpHandler) RegisterRoutes(r fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	r = r.Group("/bills", mid.AuthMiddleware)

	r.Post("/", h.CreateBill)
	r.Get("/", h.GetAllBills)
	r.Get("/:id", h.GetBill)

	a := r.Group("/admin", mid.AdminMiddleware)

	a.Get("/all", h.GetAllBillsAdmin)
	a.Put("/:id/void", h.VoidBill)
}

func mapBillError(err error) error {
	switch {
	case errors.Is(err, ErrMatchNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Match not found")
	case errors.Is(err, ErrInvalidBill):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid bill request")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	case errors.Is(err, ErrBillConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "BILL_CONFLICT", "Bill state conflicts with this operation")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Bill not found")
	default:
		return err
	}
}

// CreateBill godoc
// @Summary Place an authoritative bill
// @Description Breaking contract: money is a string and rates are calculated by the server.
// @Tags Bill
// @Accept json
// @Produce json
// @Param bill body model.CreateBillRequest true "Bill stake and selections"
// @Success 201 {object} model.BillHeadDto
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 422 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /bills [post]
// @Security BearerAuth
func (h *BillHttpHandler) CreateBill(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req model.CreateBillRequest

	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	v, err := h.service.CreateBill(u.Id, &req)

	if err != nil {
		return mapBillError(err)
	}

	return c.Status(201).JSON(v)
}

// GetBill godoc
// @Summary Get a bill by ID
// @Tags Bill
// @Produce json
// @Param id path string true "Bill ID"
// @Success 200 {object} model.BillHeadDto
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /bills/{id} [get]
// @Security BearerAuth
func (h *BillHttpHandler) GetBill(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	if c.Params("id") == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	v, e := h.service.GetBill(c.Params("id"), u.Id)

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(v)
}

// GetAllBills godoc
// @Summary Get the authenticated user's bills
// @Tags Bill
// @Produce json
// @Success 200 {array} model.BillHeadDto
// @Router /bills [get]
// @Security BearerAuth
func (h *BillHttpHandler) GetAllBills(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	v, e := h.service.GetAllBills(u.Id)

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(v)
}

// GetAllBillsAdmin godoc
// @Summary Get all bills (admin)
// @Tags Bill
// @Produce json
// @Success 200 {array} model.BillHeadDto
// @Router /bills/admin/all [get]
// @Security BearerAuth
func (h *BillHttpHandler) GetAllBillsAdmin(c *fiber.Ctx) error {
	v, e := h.service.GetAllBillsAdmin()

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(v)
}

// VoidBill godoc
// @Summary Void and refund a pending bill
// @Tags Bill
// @Accept json
// @Produce json
// @Param id path string true "Bill ID"
// @Param request body model.VoidBillRequest true "Audit reason"
// @Success 200 {object} model.BillHeadDto
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /bills/admin/{id}/void [put]
// @Security BearerAuth
func (h *BillHttpHandler) VoidBill(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	if c.Params("id") == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	var req model.VoidBillRequest

	if e := apierror.BindJSON(c, &req); e != nil {
		return e
	}

	v, e := h.service.VoidBill(c.Params("id"), u.Id, req.Reason)

	if e != nil {
		return mapBillError(e)
	}

	return c.JSON(v)
}
