// internal/middleware/auth.go
package middleware

import (
	"strings" // Digunakan untuk perbandingan string case-insensitive (EqualFold)

	"github.com/gofiber/fiber/v2"                              // Framework Fiber
	"github.com/rakaarfi/attendance-system-be/internal/models" // Model untuk struktur Response
	"github.com/rakaarfi/attendance-system-be/internal/utils"  // Utilitas untuk JWT (ExtractToken, ValidateJWT, JwtClaims)
	zlog "github.com/rs/zerolog/log"                           // Logger global Zerolog
)

// Protected adalah middleware Fiber yang memastikan sebuah request memiliki token JWT yang valid.
// Middleware ini harus dijalankan *sebelum* handler atau middleware lain yang memerlukan
// informasi user yang terautentikasi.
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// --- 1. Ekstrak Token dari Header Authorization ---
		// Mencari header "Authorization: Bearer <token>" dan mengambil bagian token-nya.
		tokenString := utils.ExtractToken(c)
		if tokenString == "" {
			// Jika token tidak ditemukan, log peringatan dan kirim response 401 Unauthorized.
			zlog.Warn().Str("path", c.Path()).Str("ip", c.IP()).Msg("Protected route access attempt without token")
			return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
				Success: false, Message: "Unauthorized: Missing token",
			})
		}

		// --- 2. Validasi Token JWT ---
		// Memverifikasi signature token, masa berlaku (expiry), dan mem-parsing claims.
		claims, err := utils.ValidateJWT(tokenString)
		if err != nil {
			// Jika token tidak valid (kadaluarsa, signature salah, format rusak), log error dan kirim 401.
			zlog.Warn().Err(err).Str("path", c.Path()).Str("ip", c.IP()).Msg("Protected route access attempt with invalid token")
			return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
				Success: false, Message: "Unauthorized: Invalid token",
			})
		}

		// --- 3. Simpan Claims ke Locals ---
		// Jika token valid, simpan data claims (*utils.JwtClaims) ke dalam context request Fiber (c.Locals).
		// Kunci "user" digunakan secara konvensi. Handler/middleware selanjutnya bisa mengambil data ini.
		c.Locals("user", claims) // Menyimpan pointer ke JwtClaims

		// --- 4. Lanjutkan ke Middleware/Handler Berikutnya ---
		// Log level debug untuk menandakan autentikasi berhasil (hanya muncul jika LOG_LEVEL=debug).
		zlog.Debug().Str("username", claims.Username).Int("user_id", claims.UserID).Str("role", claims.Role).Msg("JWT authenticated, proceeding")
		return c.Next() // Lanjutkan ke proses selanjutnya dalam rantai middleware/handler.
	}
}

// Authorize adalah middleware Fiber yang memeriksa apakah user yang terautentikasi
// memiliki salah satu role yang diizinkan untuk mengakses suatu route.
// Middleware ini WAJIB dijalankan *setelah* middleware Protected() agar claims user sudah ada di c.Locals.
//
// Parameter:
//   - allowedRoles: Daftar string nama role yang diizinkan (varargs).
func Authorize(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// --- 1. Ambil Claims User dari Locals ---
		// Mengambil data claims (*utils.JwtClaims) yang sebelumnya disimpan oleh middleware Protected().
		// Melakukan type assertion untuk memastikan tipenya benar.
		claims, ok := c.Locals("user").(*utils.JwtClaims)
		if !ok {
			// Jika claims tidak ditemukan atau tipenya salah (seharusnya tidak terjadi jika Protected() jalan duluan),
			// log error kritis dan kirim response 403 Forbidden.
			// Status 500 mungkin juga bisa dipertimbangkan karena ini menandakan kesalahan konfigurasi middleware.
			zlog.Error().Str("path", c.Path()).Str("ip", c.IP()).Msg("User claims not found in context during authorization. Ensure Protected middleware runs first.")
			return c.Status(fiber.StatusForbidden).JSON(models.Response{
				Success: false, Message: "Forbidden: Cannot determine user role",
			})
		}

		// --- 2. Periksa Role User ---
		// Iterasi melalui daftar role yang diizinkan (allowedRoles).
		isAllowed := false
		for _, role := range allowedRoles {
			// Membandingkan role user (dari claims JWT) dengan role yang diizinkan.
			// strings.EqualFold digunakan untuk perbandingan case-insensitive (misal: "Admin" == "admin").
			if strings.EqualFold(claims.Role, role) {
				isAllowed = true // Jika cocok, set flag true dan hentikan loop.
				break
			}
		}

		// --- 3. Tolak Akses Jika Role Tidak Sesuai ---
		if !isAllowed {
			// Jika role user tidak ada dalam daftar yang diizinkan, log peringatan dan kirim 403 Forbidden.
			zlog.Warn().Str("username", claims.Username).Int("user_id", claims.UserID).Str("user_role", claims.Role).Strs("required_roles", allowedRoles).Str("path", c.Path()).Msg("Authorization failed: User role not permitted")
			return c.Status(fiber.StatusForbidden).JSON(models.Response{
				Success: false, Message: "Forbidden: Insufficient privileges",
			})
		}

		// --- 4. Izinkan Akses Jika Role Sesuai ---
		// Jika user memiliki role yang diizinkan, log debug dan lanjutkan ke handler berikutnya.
		zlog.Debug().Str("username", claims.Username).Str("role", claims.Role).Msg("Authorization successful, proceeding")
		return c.Next()
	}
}
