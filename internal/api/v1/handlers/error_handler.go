package handlers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	"github.com/rs/zerolog/log"
	// Import error spesifik jika perlu dicek (misal: validator.ValidationErrors)
)

// ErrorHandler custom untuk Fiber
func ErrorHandler(ctx *fiber.Ctx, err error) error {
	// Default error code
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	// Ambil status code dari fiber.Error jika ada
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	// Handle error spesifik lain jika perlu
	// Misalnya:
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		code = fiber.StatusBadRequest
		// Format pesan error validasi
		message = "Validation Failed" // Atau buat pesan yg lebih detail
	}

	// Log error dengan zerolog (sebelumnya sudah dilog oleh middleware, tapi ini untuk detail)
	log.Error().Err(err).
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Int("status_sent", code).
		Msg("Error occurred during request processing")

	// Kirim response JSON error
	ctx.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return ctx.Status(code).JSON(models.Response{
		Success: false,
		Message: message,
		// Data: err.Error(), // Hati-hati mengirim detail error ke client
	})
}
