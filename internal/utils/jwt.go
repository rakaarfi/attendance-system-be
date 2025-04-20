// internal/utils/jwt.go
package utils

import (
	"fmt"     // Untuk formatting error dan string
	"os"      // Untuk membaca environment variable (JWT_SECRET)
	"strconv" // Untuk konversi string ke integer (ExtractUserIDFromParam)
	"strings" // Untuk manipulasi string (ExtractToken)
	"time"    // Untuk menentukan waktu kedaluwarsa token

	"github.com/gofiber/fiber/v2"    // Framework Fiber, digunakan untuk context (c *fiber.Ctx)
	"github.com/golang-jwt/jwt/v5"   // Library populer untuk membuat dan memvalidasi JWT
	zlog "github.com/rs/zerolog/log" // Logger global Zerolog
)

// JwtClaims mendefinisikan struktur data (payload) yang akan disimpan di dalam token JWT.
// Menyertakan RegisteredClaims standar JWT dan field custom (UserID, Username, Role).
type JwtClaims struct {
	UserID               int    `json:"user_id"`  // ID pengguna
	Username             string `json:"username"` // Username pengguna
	Role                 string `json:"role"`     // Role pengguna (misal: "Admin", "Employee")
	jwt.RegisteredClaims        // Menyematkan claims standar JWT (ExpiresAt, IssuedAt, Issuer, dll.)
}

// jwtSecret adalah kunci rahasia yang digunakan untuk menandatangani (sign) dan memverifikasi token JWT.
// Diambil dari environment variable "JWT_SECRET". HARUS dijaga kerahasiaannya.
// Diinisialisasi saat paket dimuat.
var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

// GenerateJWT membuat string token JWT baru yang ditandatangani untuk user tertentu.
// Menerima ID, username, dan role user sebagai input.
// Mengembalikan string token atau error jika proses signing gagal.
func GenerateJWT(userID int, username, role string) (string, error) {
	// Tentukan masa berlaku token (misal: 72 jam dari sekarang).
	expirationTime := time.Now().Add(72 * time.Hour)

	// Buat instance JwtClaims dengan data user dan claims standar.
	claims := JwtClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime), // Waktu kedaluwarsa
			IssuedAt:  jwt.NewNumericDate(time.Now()),     // Waktu token dibuat
			NotBefore: jwt.NewNumericDate(time.Now()),     // Waktu token mulai valid (biasanya sama dengan IssuedAt)
			Issuer:    "absensi-app",                      // Pengenal aplikasi yang mengeluarkan token (opsional)
			// Subject: strconv.Itoa(userID), // ID User sebagai subject (opsional)
		},
	}

	// Buat token baru dengan claims dan metode signing HS256 (HMAC SHA-256).
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Tandatangani token menggunakan jwtSecret.
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		// Log error jika signing gagal.
		zlog.Error().Err(err).Msg("Error signing JWT token")
		return "", fmt.Errorf("error signing token: %w", err) // Kembalikan error
	}

	// Log (debug) bahwa token berhasil dibuat.
	zlog.Debug().Int("user_id", userID).Str("username", username).Str("role", role).Msg("Generated JWT token")
	return signedToken, nil // Kembalikan token string
}

// ValidateJWT memverifikasi token JWT string yang diberikan.
// Mengecek signature, masa berlaku, dan mem-parsing claims ke dalam struct JwtClaims.
// Mengembalikan pointer ke JwtClaims jika valid, atau error jika tidak valid.
func ValidateJWT(tokenString string) (*JwtClaims, error) {
	// Parse token string, validasi signature & expiry, dan decode claims ke struct JwtClaims.
	token, err := jwt.ParseWithClaims(tokenString, &JwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		// --- Validasi Metode Signing ---
		// Sangat penting untuk memastikan token menggunakan algoritma yang diharapkan (HS256 dalam kasus ini).
		// Mencegah serangan penggantian algoritma (misal: ke 'none').
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// Log peringatan jika algoritma tidak sesuai.
			algo := "unknown"
			if algStr, okAlg := token.Header["alg"].(string); okAlg {
				algo = algStr
			}
			zlog.Warn().Str("algorithm", algo).Msg("Unexpected signing method during JWT validation")
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Kembalikan secret key yang benar untuk verifikasi.
		return jwtSecret, nil
	})

	// Handle error saat parsing/validasi awal (misal: format token salah, expired, signature mismatch).
	if err != nil {
		zlog.Warn().Err(err).Msg("Error parsing or validating JWT token") // Log sebagai Warn karena ini bisa terjadi karena input user
		return nil, fmt.Errorf("error parsing token: %w", err)            // Kembalikan error
	}

	// --- Cek Validitas Token dan Claims ---
	// Pastikan token secara keseluruhan valid (tidak expired, signature cocok)
	// dan claims berhasil di-decode ke struct JwtClaims.
	if claims, ok := token.Claims.(*JwtClaims); ok && token.Valid {
		// Log (debug) bahwa token valid.
		zlog.Debug().Str("username", claims.Username).Int("user_id", claims.UserID).Msg("JWT token validated successfully")
		return claims, nil // Kembalikan pointer ke claims yang valid
	}

	// Jika token tidak valid atau claims tidak bisa di-cast.
	zlog.Warn().Msg("Invalid token or claims after parsing")
	return nil, fmt.Errorf("invalid token")
}

// ExtractToken adalah fungsi helper untuk mengambil token string dari header "Authorization".
// Mengharapkan format "Bearer <token>".
// Mengembalikan token string atau string kosong jika header tidak ada atau formatnya salah.
func ExtractToken(c *fiber.Ctx) string {
	authHeader := c.Get(fiber.HeaderAuthorization) // Ambil nilai header "Authorization"
	if authHeader == "" {
		return "" // Tidak ada header
	}

	// Pisahkan string header berdasarkan spasi ("Bearer", "<token>")
	parts := strings.Split(authHeader, " ")
	// Periksa apakah formatnya benar (harus ada 2 bagian, bagian pertama "Bearer")
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		// Log (debug) token yang diekstrak.
		// zlog.Debug().Msg("Extracted token from Authorization header") // Mungkin terlalu verbose
		return parts[1] // Kembalikan bagian kedua (token)
	}

	// Log peringatan jika format header salah.
	zlog.Warn().Str("AuthorizationHeader", authHeader).Msg("Invalid Authorization header format (Expected 'Bearer <token>')")
	return "" // Format salah
}

// ExtractUserIDFromJWT adalah fungsi helper untuk mengambil UserID dari context Fiber.
// Mengasumsikan middleware Protected() sudah berjalan dan menyimpan *JwtClaims di c.Locals("user").
// Mengembalikan UserID atau error jika claims tidak ditemukan/tipenya salah.
func ExtractUserIDFromJWT(c *fiber.Ctx) (int, error) {
	// Ambil data dari Locals dan lakukan type assertion ke *JwtClaims.
	claims, ok := c.Locals("user").(*JwtClaims)
	if !ok {
		// Log error jika claims tidak ditemukan atau tipe salah (menandakan masalah aliran middleware).
		zlog.Error().Str("path", c.Path()).Msg("Could not extract user claims from Fiber context (middleware issue?)")
		return 0, fmt.Errorf("could not extract user claims from context")
	}
	// Log (debug) ID yang diekstrak.
	// zlog.Debug().Int("user_id", claims.UserID).Msg("Extracted UserID from JWT context")
	return claims.UserID, nil
}

// ExtractRoleFromJWT adalah fungsi helper untuk mengambil Role dari context Fiber.
// Sama seperti ExtractUserIDFromJWT, mengasumsikan Protected() sudah berjalan.
// Mengembalikan string Role atau error.
func ExtractRoleFromJWT(c *fiber.Ctx) (string, error) {
	claims, ok := c.Locals("user").(*JwtClaims)
	if !ok {
		zlog.Error().Str("path", c.Path()).Msg("Could not extract user claims from Fiber context (middleware issue?)")
		return "", fmt.Errorf("could not extract user claims from context")
	}
	// zlog.Debug().Str("role", claims.Role).Msg("Extracted Role from JWT context")
	return claims.Role, nil
}

// ExtractUserIDFromParam adalah fungsi helper untuk mengambil UserID dari parameter path URL.
// Contoh: /users/{userId} -> c.Params("userId").
// Mengembalikan UserID (int) atau error jika parameter tidak ada atau bukan angka valid.
func ExtractUserIDFromParam(c *fiber.Ctx, paramName string) (int, error) { // Tambahkan paramName
	idStr := c.Params(paramName) // Gunakan nama parameter yang dinamis
	if idStr == "" {
		zlog.Warn().Str("paramName", paramName).Str("path", c.Path()).Msg("Missing User ID parameter in URL path")
		return 0, fmt.Errorf("missing user ID parameter '%s'", paramName)
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		// Log warning jika parameter bukan angka.
		zlog.Warn().Err(err).Str("paramName", paramName).Str("value", idStr).Str("path", c.Path()).Msg("Invalid numeric value for User ID parameter")
		return 0, fmt.Errorf("invalid user ID parameter '%s': not a number", paramName)
	}
	return id, nil // Kembalikan ID integer
}
