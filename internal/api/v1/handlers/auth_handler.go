package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	"github.com/rakaarfi/attendance-system-be/internal/repository"
	"github.com/rakaarfi/attendance-system-be/internal/utils"
	zlog "github.com/rs/zerolog/log"
)

type AuthHandler struct {
	UserRepo repository.UserRepository
	RoleRepo repository.RoleRepository
	Validate *validator.Validate
}

func NewAuthHandler(userRepo repository.UserRepository, roleRepo repository.RoleRepository) *AuthHandler {
	return &AuthHandler{
		UserRepo: userRepo,
		RoleRepo: roleRepo,
		Validate: validator.New(),
	}
}

// Register godoc
// @Summary Register New User
// @Description Creates a new user account.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param register body models.RegisterUserInput true "User Registration Details"
// @Success 201 {object} models.Response{data=map[string]int} "User registered successfully, returns user ID"
// @Failure 400 {object} models.Response "Validation failed or invalid request body"
// @Failure 409 {object} models.Response "Username or Email already exists" // Tambahkan jika ada penanganan conflict
// @Failure 500 {object} models.Response "Internal server error during registration"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	input := new(models.RegisterUserInput)

	// Parse body
	if err := c.BodyParser(input); err != nil {
		zlog.Error().Err(err).Msg("Error parsing register input")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Validate input
	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Msg("Validation failed during registration") // Log warning
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false,
			Message: "Validation failed",
			Data:    err.Error(), // Memberikan detail error (hati-hati dengan info sensitif)
		})
	}

	// --- Optional: Validasi Role ID ---
	_, err := h.RoleRepo.GetRoleByID(context.Background(), input.RoleID)
	if err != nil {
		log.Printf("Error getting role ID %d: %v", input.RoleID, err)
		// Handle jika role tidak ditemukan (pgx.ErrNoRows)
		if errors.Is(err, pgx.ErrNoRows) {
			return c.Status(fiber.StatusBadRequest).JSON(models.Response{
				Success: false,
				Message: fmt.Sprintf("Role with ID %d not found", input.RoleID),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false,
			Message: "Failed to validate role",
		})
	}
	// --- End Optional ---

	// Hash password
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		zlog.Warn().Err(err).Msg("Validation failed during registration") // Log warning
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false,
			Message: "Failed to process registration",
		})
	}
	// --- CETAK HASH SAAT REGISTRASI (Sementara) ---
	zlog.Debug().Str("username", input.Username).Str("plaintext", input.Password).Str("generated_hash", hashedPassword).Msg("Password hashed during registration")
	// --- AKHIR CETAK ---

	// Create user in database
	zlog.Debug().Str("username", input.Username).Msg("Attempting to create user in DB") // Log debug
	userID, err := h.UserRepo.CreateUser(context.Background(), input, hashedPassword)
	if err != nil {
		zlog.Error().Err(err).Str("username", input.Username).Msg("Error creating user in DB")
		// Cek error spesifik (misal: username/email sudah ada - unique constraint violation)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return c.Status(fiber.StatusConflict).JSON(models.Response{ // 409 Conflict
				Success: false,
				Message: "Username or Email already exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false,
			Message: "Failed to register user",
		})
	}

	zlog.Info().Int("userID", userID).Str("username", input.Username).Msg("User registered successfully")
	// Jangan kirim data user lengkap atau password hash di response registrasi
	return c.Status(fiber.StatusCreated).JSON(models.Response{
		Success: true,
		Message: "User registered successfully",
		Data:    fiber.Map{"user_id": userID},
	})
}

// Login godoc
// @Summary User Login
// @Description Authenticates a user and returns a JWT token upon successful login.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param login body models.LoginUserInput true "Login Credentials"
// @Success 200 {object} models.Response{data=map[string]string} "Login successful, returns JWT token"
// @Failure 400 {object} models.Response "Validation failed or invalid request body"
// @Failure 401 {object} models.Response "Invalid username or password"
// @Failure 500 {object} models.Response "Internal server error during login"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	input := new(models.LoginUserInput)

	if err := c.BodyParser(input); err != nil {
		zlog.Warn().Err(err).Msg("Invalid request body during login")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid request body",
		})
	}

	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Msg("Validation failed during login")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	// Get user by username
	user, err := h.UserRepo.GetUserByUsername(context.Background(), input.Username)
	if err != nil {
		zlog.Error().Err(err).Str("username", input.Username).Msg("Error getting user during login")
		if err == pgx.ErrNoRows { // User tidak ditemukan
			zlog.Info().Str("username", input.Username).Msg("User not found during login")
			return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
				Success: false, Message: "Invalid username or password",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Login failed",
		})
	}

	// Check password
	if !utils.CheckPasswordHash(input.Password, user.Password) {
		zlog.Info().Str("username", input.Username).Msg("Invalid password during login")
		return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
			Success: false, Message: "Invalid username or password",
		})
	}

	// Generate JWT
	if user.Role == nil { // Pastikan role sudah di-load
		zlog.Warn().Int("user_id", user.ID).Msg("Role not loaded for user during login")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Login failed: User role missing",
		})
	}
	token, err := utils.GenerateJWT(user.ID, user.Username, user.Role.Name) // Gunakan nama role
	if err != nil {
		zlog.Error().Err(err).Str("username", input.Username).Msg("Error generating JWT for user during login")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Login failed",
		})
	}

	zlog.Info().Str("username", input.Username).Msg("User logged in successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true,
		Message: "Login successful",
		Data:    fiber.Map{"token": token},
	})
}
