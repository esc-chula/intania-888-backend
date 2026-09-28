package apierror

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RequestIDHeader carries the server-generated correlation ID for an API request.
const RequestIDHeader = "X-Request-ID"

// Error is an application error with a stable public code and message.
// Cause is retained for errors.Is/errors.As and server-side diagnostics.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	Cause   error             `json:"-"`
}

// Error formats diagnostic code and cause; HTTP rendering uses the safe Message field.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return e.Code + ": " + e.Message
}

// Unwrap exposes the original failure for errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// New constructs a safe public failure without an internal cause.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Wrap retains a diagnostic cause while exposing only the supplied public message.
func Wrap(cause error, status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message, Cause: cause}
}

// Invalid reports field-specific request validation failures using the shared 400 contract.
func Invalid(details map[string]string) *Error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    "INVALID_REQUEST",
		Message: "Invalid request",
		Details: details,
	}
}

// Response is the public failure envelope; internal causes never enter the JSON body.
type Response struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Details   map[string]string `json:"details,omitempty"`
}

// RequestID installs a server-generated ID before any API middleware can reject a request.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := uuid.NewString()
		c.Locals("request_id", id)
		c.Set(RequestIDHeader, id)

		return c.Next()
	}
}

func requestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok && id != "" {
		return id
	}

	id := uuid.NewString()
	c.Locals("request_id", id)
	c.Set(RequestIDHeader, id)

	return id
}

// ErrorHandler is the single Fiber error mapper for all API failures.
func ErrorHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		id := requestID(c)
		status := http.StatusInternalServerError
		code := "INTERNAL_ERROR"
		message := "Internal server error"

		var details map[string]string
		var appErr *Error

		if errors.As(err, &appErr) {
			status, code, message, details = appErr.Status, appErr.Code, appErr.Message, appErr.Details

			if log != nil {
				fields := []zap.Field{
					zap.String("request_id", id),
					zap.String("code", appErr.Code),
					zap.Int("status", appErr.Status),
				}

				if appErr.Cause != nil {
					fields = append(fields, zap.Error(appErr.Cause))
				}

				log.Warn("API request failed", fields...)
			}
		} else {
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				status = fiberErr.Code
				code, message = publicFailure(status)
			}

			if status >= http.StatusInternalServerError && log != nil {
				log.Error("Unhandled API error", zap.String("request_id", id), zap.Error(err))
			}
		}

		if status < 400 || status > 599 {
			status, code, message = http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error"
		}

		c.Set(RequestIDHeader, id)

		return c.Status(status).JSON(Response{Code: code, Message: message, RequestID: id, Details: details})
	}
}

func publicFailure(status int) (string, string) {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_REQUEST", "Invalid request"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED", "Authentication required"
	case http.StatusForbidden:
		return "FORBIDDEN", "Permission denied"
	case http.StatusNotFound:
		return "RESOURCE_NOT_FOUND", "Resource not found"
	case http.StatusConflict:
		return "CONFLICT", "The request conflicts with the current state"
	case http.StatusUnprocessableEntity:
		return "INVALID_REQUEST", "The request cannot be processed"
	case http.StatusTooManyRequests:
		return "TOO_MANY_REQUESTS", "Too many requests"
	case http.StatusMethodNotAllowed:
		return "INVALID_REQUEST", "Method not allowed"
	case http.StatusServiceUnavailable:
		return "DEPENDENCY_UNAVAILABLE", "A required service is unavailable"
	default:
		if status >= 400 && status < 500 {
			return "INVALID_REQUEST", "The request cannot be processed"
		}
		return "INTERNAL_ERROR", "Internal server error"
	}
}

// BindJSON decodes exactly one JSON object, rejects unknown fields, and runs
// request DTO validation when dst implements ValidateRequest.
func BindJSON(c *fiber.Ctx, dst any) error {
	if err := decodeStrict(c.Body(), dst); err != nil {
		return Invalid(map[string]string{"body": "must be one valid JSON object with known fields"})
	}

	if validator, ok := dst.(interface{ ValidateRequest() map[string]string }); ok {
		if details := validator.ValidateRequest(); len(details) > 0 {
			return Invalid(details)
		}
	}

	return nil
}
