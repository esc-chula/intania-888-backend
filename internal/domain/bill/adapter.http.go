package bill

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
)

type BillHttpHandler struct {
	service BillService
}

func NewBillHttpHandler(service BillService) *BillHttpHandler {
	return &BillHttpHandler{service}
}

func (h *BillHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/bills", mid.AuthMiddleware)

	router.Post("/", h.CreateBill)
	router.Get("/", h.GetAllBills)
	router.Get("/:id", h.GetBill)
	router.Patch("/:id", h.UpdateBill)
	router.Delete("/:id", h.DeleteBill)
}

// CreateBill godoc
// @Summary Create a new bill
// @Description Create a new bill with the input payload
// @Tags Bill
// @Accept json
// @Produce json
// @Param bill body model.BillHeadDto true "Create bill"
// @Success 201 {object} model.BillHeadDto
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /bills [post]
func (h *BillHttpHandler) CreateBill(c *fiber.Ctx) error {
	// get user from context
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errors.New("not found user profile in context").Error()})
	}

	var billDto model.BillHeadDto
	if err := c.BodyParser(&billDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "Invalid request payload"})
	}

	billDto.UserId = userProfile.Id
	err := h.service.CreateBill(userProfile, &billDto)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to create bill"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "created bill successful"})
}

// GetBill godoc
// @Summary Get a bill by ID
// @Description Get a bill by its ID
// @Tags Bill
// @Accept json
// @Produce json
// @Param id path string true "Bill ID"
// @Success 200 {object} model.BillHeadDto
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /bills/{id} [get]
func (h *BillHttpHandler) GetBill(c *fiber.Ctx) error {
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errors.New("not found user profile in context").Error()})
	}

	bill, err := h.service.GetBill(c.Params("id"), userProfile.Id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Message: "Bill not found"})
	}

	return c.JSON(bill)
}

// GetAllBills godoc
// @Summary Get all bills
// @Description Get all bills
// @Tags Bill
// @Accept json
// @Produce json
// @Success 200 {array} model.BillHeadDto
// @Failure 500 {object} ErrorResponse
// @Router /bills [get]
func (h *BillHttpHandler) GetAllBills(c *fiber.Ctx) error {
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errors.New("not found user profile in context").Error()})
	}

	bills, err := h.service.GetAllBills(userProfile.Id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to get bills"})
	}

	return c.JSON(bills)
}

// UpdateBill godoc
// @Summary Update a bill
// @Description Update a bill with the input payload
// @Tags Bill
// @Accept json
// @Produce json
// @Param id path string true "Bill ID"
// @Param bill body model.BillHeadDto true "Update bill"
// @Success 200 {object} model.BillHeadDto
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /bills/{id} [put]
func (h *BillHttpHandler) UpdateBill(c *fiber.Ctx) error {
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errors.New("not found user profile in context").Error()})
	}

	var billDto model.BillHeadDto
	if err := c.BodyParser(&billDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "Invalid request payload"})
	}
	if billDto.UserId != userProfile.Id || billDto.Id != c.Params("id") {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Message: "User is not allowed to access this bill"})
	}

	err := h.service.UpdateBill(&billDto)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to update bill"})
	}

	return c.JSON(billDto)
}

// DeleteBill godoc
// @Summary Delete a bill
// @Description Delete a bill by its ID
// @Tags Bill
// @Accept json
// @Produce json
// @Param id path string true "Bill ID"
// @Success 204 "No Content"
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /bills/{id} [delete]
func (h *BillHttpHandler) DeleteBill(c *fiber.Ctx) error {
	id := c.Params("id")

	err := h.service.DeleteBill(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to delete bill"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

type ErrorResponse struct {
	Message string `json:"message"`
}
