package bill

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

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

func strictJSON(c *fiber.Ctx, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}

	return nil
}

func billError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrInvalidBill):
		return c.Status(400).JSON(ErrorResponse{"Invalid request payload"})
	case errors.Is(err, ErrInsufficientBalance):
		return c.Status(422).JSON(ErrorResponse{"Insufficient balance"})
	case errors.Is(err, ErrBillConflict):
		return c.Status(409).JSON(ErrorResponse{"Bill is already settled"})
	case errors.Is(err, gorm.ErrRecordNotFound):
		return c.Status(404).JSON(ErrorResponse{"Bill not found"})
	default:
		return c.Status(500).JSON(ErrorResponse{"Bill operation failed"})
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
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Router /bills [post]
// @Security BearerAuth
func (h *BillHttpHandler) CreateBill(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return c.Status(400).JSON(ErrorResponse{"User profile missing"})
	}

	var req model.CreateBillRequest

	if err := strictJSON(c, &req); err != nil {
		return c.Status(400).JSON(ErrorResponse{"Invalid request payload"})
	}

	v, err := h.service.CreateBill(u.Id, &req)

	if err != nil {
		return billError(c, err)
	}

	return c.Status(201).JSON(v)
}

// GetBill godoc
// @Summary Get a bill by ID
// @Tags Bill
// @Produce json
// @Param id path string true "Bill ID"
// @Success 200 {object} model.BillHeadDto
// @Router /bills/{id} [get]
// @Security BearerAuth
func (h *BillHttpHandler) GetBill(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return c.SendStatus(400)
	}

	v, e := h.service.GetBill(c.Params("id"), u.Id)

	if e != nil {
		return billError(c, e)
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
		return c.SendStatus(400)
	}

	v, e := h.service.GetAllBills(u.Id)

	if e != nil {
		return billError(c, e)
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
		return billError(c, e)
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
// @Failure 409 {object} ErrorResponse
// @Router /bills/admin/{id}/void [put]
// @Security BearerAuth
func (h *BillHttpHandler) VoidBill(c *fiber.Ctx) error {
	u := utils.GetUserProfileFromCtx(c)

	if u == nil {
		return c.SendStatus(401)
	}

	var req model.VoidBillRequest

	if e := strictJSON(c, &req); e != nil {
		return c.Status(400).JSON(ErrorResponse{"Invalid request payload"})
	}

	v, e := h.service.VoidBill(c.Params("id"), u.Id, req.Reason)

	if e != nil {
		return billError(c, e)
	}

	return c.JSON(v)
}

type ErrorResponse struct {
	Message string `json:"message"`
}
