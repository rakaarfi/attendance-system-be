package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	zlog "github.com/rs/zerolog/log"
)

// Claims custom untuk JWT
type JwtClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // Nama role bisa dimasukkan ke claim
	jwt.RegisteredClaims
}

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func GenerateJWT(userID int, username, role string) (string, error) {
	claims := JwtClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)), // Token berlaku 72 jam
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "absensi-app",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		zlog.Error().Err(err).Msg("Error signing token")
		return "", fmt.Errorf("error signing token: %w", err)
	}
	zlog.Debug().Int("user_id", userID).Str("username", username).Str("role", role).Msg("Generated JWT token")
	return signedToken, nil
}

func ValidateJWT(tokenString string) (*JwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Pastikan metode signing adalah HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			zlog.Warn().Str("algorithm", token.Header["alg"].(string)).Msg("Unexpected signing method")
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		zlog.Error().Err(err).Msg("Error parsing token")
		return nil, fmt.Errorf("error parsing token: %w", err)
	}

	if claims, ok := token.Claims.(*JwtClaims); ok && token.Valid {
		zlog.Debug().Str("username", claims.Username).Int("user_id", claims.UserID).Msg("Valid token")
		return claims, nil
	}

	zlog.Warn().Str("token", tokenString).Msg("Invalid token")
	return nil, fmt.Errorf("invalid token")
}

// ExtractToken helper untuk mengambil token dari header Authorization
func ExtractToken(c *fiber.Ctx) string {
	bearToken := c.Get("Authorization")
	// Formatnya: Bearer <token>
	onlyToken := strings.Split(bearToken, " ")
	if len(onlyToken) == 2 {
		zlog.Debug().Str("token", onlyToken[1]).Msg("Extracted token from Authorization header")
		return onlyToken[1]
	}
	zlog.Warn().Str("Authorization", bearToken).Msg("Invalid Authorization header format")
	return ""
}

// ExtractUserIDFromJWT mengambil UserID dari context Fiber (setelah middleware auth)
func ExtractUserIDFromJWT(c *fiber.Ctx) (int, error) {
	claims, ok := c.Locals("user").(*JwtClaims)
	if !ok {
		zlog.Warn().Msg("Could not extract user claims from context")
		return 0, fmt.Errorf("could not extract user claims from context")
	}
	zlog.Debug().Int("user_id", claims.UserID).Msg("Extracted user ID from JWT")
	return claims.UserID, nil
}

// ExtractRoleFromJWT mengambil Role dari context Fiber
func ExtractRoleFromJWT(c *fiber.Ctx) (string, error) {
	claims, ok := c.Locals("user").(*JwtClaims)
	if !ok {
		zlog.Warn().Msg("Could not extract user claims from context")
		return "", fmt.Errorf("could not extract user claims from context")
	}
	zlog.Debug().Str("role", claims.Role).Msg("Extracted role from JWT")
	return claims.Role, nil
}

// ExtractUserIDFromParam mengambil UserID dari parameter URL
func ExtractUserIDFromParam(c *fiber.Ctx) (int, error) {
	idStr := c.Params("userId") // Sesuaikan nama param jika berbeda
	id, err := strconv.Atoi(idStr)
	if err != nil {
		zlog.Warn().Err(err).Msgf("Invalid user ID parameter: %v", idStr)
		return 0, fmt.Errorf("invalid user ID parameter: %w", err)
	}
	return id, nil
}
