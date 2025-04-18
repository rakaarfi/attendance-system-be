package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	"github.com/rakaarfi/attendance-system-be/internal/utils"
	zlog "github.com/rs/zerolog/log" 
)

// Protected adalah middleware untuk melindungi route yang memerlukan autentikasi
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenString := utils.ExtractToken(c)
		if tokenString == "" {
			zlog.Warn().Msg("Missing token in request")
			return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
				Success: false, Message: "Unauthorized: Missing token",
			})
		}

		claims, err := utils.ValidateJWT(tokenString)
		if err != nil {
			zlog.Error().Err(err).Msg("JWT Validation Error")
			return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
				Success: false, Message: "Unauthorized: Invalid token",
			})
		}

		// Simpan informasi user (claims) di context Fiber untuk digunakan handler selanjutnya
		c.Locals("user", claims)

		// Lanjutkan ke handler berikutnya
		zlog.Debug().Msg("Valid token, proceeding to next handler")
		return c.Next()
	}
}

// Authorize adalah middleware untuk memeriksa role user
func Authorize(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Ambil claims dari context (harus dijalankan setelah Protected())
		claims, ok := c.Locals("user").(*utils.JwtClaims)
		if !ok {
			zlog.Error().Msg("User claims not found in context. Ensure Protected middleware runs first.")
			return c.Status(fiber.StatusForbidden).JSON(models.Response{
				Success: false, Message: "Forbidden: Cannot determine user role",
			})
		}

		// Cek apakah role user ada di daftar role yang diizinkan
		isAllowed := false
		for _, role := range allowedRoles {
			if strings.EqualFold(claims.Role, role) { // Gunakan EqualFold untuk case-insensitive
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			zlog.Warn().Str("username", claims.Username).Str("role", claims.Role).Strs("allowedRoles", allowedRoles).
				Msg("Forbidden access for user to resource requiring roles")
			return c.Status(fiber.StatusForbidden).JSON(models.Response{
				Success: false, Message: "Forbidden: Insufficient privileges",
			})
		}

		// User memiliki role yang diizinkan, lanjutkan
		zlog.Debug().Str("username", claims.Username).Str("role", claims.Role).
			Msg("User has allowed role, proceeding to next handler")
		return c.Next()
	}
}
