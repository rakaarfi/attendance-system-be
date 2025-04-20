package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	"github.com/rakaarfi/attendance-system-be/internal/repository"
	"github.com/rakaarfi/attendance-system-be/internal/utils"
	zlog "github.com/rs/zerolog/log"
)

type UserHandler struct {
	AttendanceRepo repository.AttendanceRepository
	ScheduleRepo   repository.ScheduleRepository
	UserRepo       repository.UserRepository
	ShiftRepo      repository.ShiftRepository
	Validate       *validator.Validate
}

func NewUserHandler(attRepo repository.AttendanceRepository, schedRepo repository.ScheduleRepository, userRepo repository.UserRepository, shiftRepo repository.ShiftRepository) *UserHandler {
	return &UserHandler{
		AttendanceRepo: attRepo,
		ScheduleRepo:   schedRepo,
		UserRepo:       userRepo,
		ShiftRepo:      shiftRepo,
		Validate:       validator.New(),
	}
}

func (h *UserHandler) CheckIn(c *fiber.Ctx) error {
	userID, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	input := new(models.CheckInInput)
	if err := c.BodyParser(input); err != nil {
		// Allow empty body for check-in without notes
		zlog.Warn().Err(err).Msg("Check-in body parsing warning (may be empty)")
	}
	// No validation needed for CheckInInput struct currently

	now := time.Now()

	// 1. Check if user has an existing attendance record without checkout
	lastAtt, err := h.AttendanceRepo.GetLastAttendance(context.Background(), userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// Handle errors other than "no attendance records at all"
		zlog.Error().Err(err).Int("user_id", userID).Msg("Error checking last attendance")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to process check-in",
		})
	}

	// If a record exists and checkout is null, prevent double check-in
	if lastAtt != nil && lastAtt.CheckOutAt == nil {
		return c.Status(fiber.StatusConflict).JSON(models.Response{
			Success: false, Message: "User already checked in",
		})
	}

	// 2. (Optional) Check if user has a schedule for today
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	_, errSched := h.ScheduleRepo.GetScheduleByUserAndDate(context.Background(), userID, today)
	if errSched != nil {
		// Handle if schedule not found vs other errors
		if errors.Is(errSched, pgx.ErrNoRows) {
			zlog.Info().Int("user_id", userID).Time("today", today).Msg("User checking in without a schedule for today")
			// Decide whether to allow check-in without schedule or return error
			return c.Status(fiber.StatusForbidden).JSON(models.Response{Success: false, Message: "No schedule found for today"})
		} else {
			zlog.Error().Err(errSched).Int("user_id", userID).Msg("Error checking schedule")
			// Maybe still allow checkin? Or return server error?
		}
	}
	// // (Optional) Validate check-in time against schedule start time?

	// 3. Proceed to check-in
	attendanceID, err := h.AttendanceRepo.CreateCheckIn(context.Background(), userID, now, input.Notes)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Time("check_in_at", now).Msg("Error creating check-in")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to record check-in",
		})
	}

	zlog.Info().Int("user_id", userID).Int("attendance_id", attendanceID).Time("check_in_at", now).Msg("Check-in successful")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Check-in successful", Data: fiber.Map{"attendance_id": attendanceID, "check_in_at": now},
	})
}

func (h *UserHandler) CheckOut(c *fiber.Ctx) error {
	userID, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	input := new(models.CheckOutInput)
	if err := c.BodyParser(input); err != nil {
		// Allow empty body for check-out without notes
		zlog.Warn().Err(err).Msg("Check-out body parsing warning (may be empty)")
	}
	// No validation needed for CheckOutInput struct currently

	now := time.Now()

	// 1. Find the last attendance record for the user that hasn't been checked out
	lastAtt, err := h.AttendanceRepo.GetLastAttendance(context.Background(), userID)
	if err != nil {
		// Handle "no records found" or other errors
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Info().Int("user_id", userID).Msg("No active check-in found to check out from")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: "No active check-in found to check out from",
			})
		}
		zlog.Error().Err(err).Int("user_id", userID).Msg("Error finding last attendance for user checkout")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to process check-out",
		})
	}

	// 2. Check if already checked out
	if lastAtt.CheckOutAt != nil {
		zlog.Info().Int("user_id", userID).Msg("User has already checked out for the last session")
		return c.Status(fiber.StatusConflict).JSON(models.Response{
			Success: false, Message: "User has already checked out for the last session",
		})
	}

	// 3. (Optional) Validate check-out time against schedule end time?

	// 4. Proceed to check-out by updating the last record
	err = h.AttendanceRepo.UpdateCheckOut(context.Background(), lastAtt.ID, now, input.Notes)
	if err != nil {
		zlog.Error().Err(err).Int("attendance_id", lastAtt.ID).Msg("Error updating check-out for attendance ID")
		// Handle specific error from repo (e.g., already checked out)
		if err.Error() == fmt.Sprintf("attendance record %d not found or already checked out", lastAtt.ID) {
			zlog.Info().Int("attendance_id", lastAtt.ID).Msg(err.Error())
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to record check-out",
		})
	}

	zlog.Info().Int("user_id", userID).Int("attendance_id", lastAtt.ID).Time("check_out_at", now).Msg("Check-out successful")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Check-out successful", Data: fiber.Map{"attendance_id": lastAtt.ID, "check_out_at": now},
	})
}

func (h *UserHandler) GetMyAttendance(c *fiber.Ctx) error {
	userID, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	// 1. Parse Tanggal
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	startDate := parseDateQueryParam(c, "start_date", startOfMonth)
	endDate := parseDateQueryParam(c, "end_date", todayEnd)

	if endDate.Before(startDate) {
		zlog.Warn().Time("start_date", startDate).Time("end_date", endDate).Msg("Invalid date range")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "end_date cannot be before start_date",
		})
	}

	zlog.Info().Int("user_id", userID).Time("start_date", startDate).Time("end_date", endDate).Msg("Retrieving attendance records for user")

	// 2. Parse Pagination Params
	pagination := utils.ParsePaginationParams(c)

	// 3. Panggil Repository
	attendances, totalCount, err := h.AttendanceRepo.GetAttendancesByUser(context.Background(), userID, startDate, endDate, pagination.Page, pagination.Limit)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to get my attendance from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve attendance records",
		})
	}

	// 4. Bangun Metadata dan Response
	meta := utils.BuildPaginationMeta(totalCount, pagination.Limit, pagination.Page)
	response := utils.NewPaginatedResponse("Attendance records retrieved successfully", attendances, meta)

	zlog.Info().Int("user_id", userID).Int("count", len(attendances)).Int("total", totalCount).Msg("Successfully retrieved my attendance")
	return c.Status(http.StatusOK).JSON(response)
}

func (h *UserHandler) GetMySchedules(c *fiber.Ctx) error {
	userID, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	// 1. Parse Tanggal
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	startDate := parseDateQueryParam(c, "start_date", startOfMonth)
	endDate := parseDateQueryParam(c, "end_date", endOfMonth)

	if endDate.Before(startDate) {
		zlog.Warn().Time("start_date", startDate).Time("end_date", endDate).Msg("Invalid date range")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "end_date cannot be before start_date",
		})
	}

	// 2. Parse Pagination Params
	pagination := utils.ParsePaginationParams(c) // Gunakan helper

	schedules, totalCount, err := h.ScheduleRepo.GetSchedulesByUser(context.Background(), userID, startDate, endDate, pagination.Page, pagination.Limit)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to get my schedules from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve schedule records",
		})
	}

	// 4. Bangun Metadata dan Response
	meta := utils.BuildPaginationMeta(totalCount, pagination.Limit, pagination.Page)
	response := utils.NewPaginatedResponse("Schedules retrieved successfully", schedules, meta) // Gunakan helper response jika ada

	zlog.Info().Int("user_id", userID).Int("count", len(schedules)).Int("total", totalCount).Msg("Successfully retrieved my schedules")
	return c.Status(http.StatusOK).JSON(response)
}

func (h *UserHandler) UpdateMyProfile(c *fiber.Ctx) error {
	// 1. Dapatkan ID user dari JWT (bukan dari URL)
	userID, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT for profile update")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	// 2. Parse & Validasi Input Body
	input := new(models.UpdateProfileInput)
	if err := c.BodyParser(input); err != nil {
		zlog.Error().Err(err).Msg("Error parsing update profile request body")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Failed to parse request body",
		})
	}

	// 3. Validasi data input menggunakan validator
	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Int("user_id", userID).Msg("Update profile validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	// 4. Panggil repository untuk update profil
	err = h.UserRepo.UpdateUserProfile(context.Background(), userID, input) // Gunakan userID dari JWT
	if err != nil {
		// Cek error unique constraint
		if strings.Contains(err.Error(), "already exists") {
			zlog.Warn().Err(err).Int("user_id", userID).Msg("Unique constraint violation during user profile update")
			return c.Status(fiber.StatusConflict).JSON(models.Response{ // 409 Conflict
				Success: false, Message: err.Error(),
			})
		}
		// Cek error user not found (seharusnya jarang terjadi di sini)
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Error().Err(err).Int("user_id", userID).Msg("User not found during profile update (inconsistency?)")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: "User not found",
			})
		}

		// Error lain saat update
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to update user profile")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to update profile",
		})
	}

	// 5. Kirim response sukses
	zlog.Info().Int("user_id", userID).Msg("User profile updated successfully")
	// Pertimbangkan untuk mengembalikan data profil yang sudah diupdate
	// (ambil lagi dari DB atau kembalikan input yang sudah divalidasi?)
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Profile updated successfully",
	})
}

func (h *UserHandler) UpdateMyPassword(c *fiber.Ctx) error {
	// 1. Dapatkan ID user dari JWT
	userID, err := utils.ExtractUserIDFromJWT(c)
	zlog.Debug().Int("jwt_user_id", userID).Err(err).Msg("Extracted User ID from JWT")
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT for password update")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	// 2. Parse & Validasi Input Body
	input := new(models.UpdatePasswordInput)
	if err := c.BodyParser(input); err != nil {
		zlog.Error().Err(err).Msg("Error parsing update password request body")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Failed to parse request body",
		})
	}

	// 3. Validasi data input menggunakan validator
	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Int("user_id", userID).Msg("Update password validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	// 4. Dapatkan data user saat ini (termasuk hash password lama) dari repo
	// Perlu method GetUserByID di UserRepo Anda!
	currentUser, err := h.UserRepo.GetUserByID(context.Background(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Error().Err(err).Int("user_id", userID).Msg("User not found during password update (inconsistency?)")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{Success: false, Message: "User not found"})
		}
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to get current user data for password check")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to process password update",
		})
	}
	zlog.Debug().
		Int("user_id", userID).
		Str("input_password", input.OldPassword).
		Str("stored_hash", currentUser.Password).
		Msg("Password update attempt details")

	// 5. Verify old password
	isMatch := utils.CheckPasswordHash(input.OldPassword, currentUser.Password)
	zlog.Debug().Bool("password_match", isMatch).Msg("Result of CheckPasswordHash")
	if !isMatch {
		zlog.Warn().Int("user_id", userID).Msg("Incorrect old password provided")
		return c.Status(fiber.StatusUnauthorized).JSON(models.Response{
			Success: false, Message: "Incorrect old password",
		})
	}

	// 6. Hash password baru
	newHashedPassword, err := utils.HashPassword(input.NewPassword)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to hash new password")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to process password update",
		})
	}

	// 7. Panggil repository untuk update password dengan hash baru
	err = h.UserRepo.UpdateUserPassword(context.Background(), userID, newHashedPassword)
	if err != nil {
		// Cek not found (seharusnya jarang)
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Error().Err(err).Int("user_id", userID).Msg("User disappeared during password update?")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{Success: false, Message: "User not found"})
		}
		// Error lain
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to update password in repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to update password",
		})
	}

	// 8. Kirim response sukses
	zlog.Info().Int("user_id", userID).Msg("User password updated successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Password updated successfully",
	})
}

func (h *UserHandler) GetMyProfile(c *fiber.Ctx) error {
	// 1. Dapatkan ID user dari JWT
	userID, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Error extracting userID from JWT for get profile")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify user",
		})
	}

	// 2. Panggil repository untuk mendapatkan data user
	// Kita gunakan GetUserByID yang mengambil user *beserta role*-nya
	userProfile, err := h.UserRepo.GetUserByID(context.Background(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Ini sangat aneh jika terjadi karena ID dari token JWT yang valid
			zlog.Error().Err(err).Int("user_id", userID).Msg("User from valid JWT not found in DB for get profile")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: "User profile not found",
			})
		}
		// Error lain
		zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to get user profile from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve profile",
		})
	}

	// 3. Kirim response sukses (password sudah otomatis tidak ada karena repo GetUserByID tidak memilihnya)
	zlog.Info().Int("user_id", userID).Msg("User profile retrieved successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Profile retrieved successfully", Data: userProfile, // Kirim data user
	})
}

func (h *UserHandler) GetAllShifts(c *fiber.Ctx) error {
    // Dapatkan ID user dari JWT (walaupun tidak dipakai di query, baik untuk log/konteks)
    userID, _ := utils.ExtractUserIDFromJWT(c) // Abaikan error jika hanya untuk log

    shifts, err := h.ShiftRepo.GetAllShifts(context.Background())
    if err != nil {
        zlog.Error().Err(err).Int("user_id", userID).Msg("Failed to get all shifts from repository")
        return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
            Success: false, Message: "Failed to retrieve shifts",
        })
    }

    zlog.Info().Int("user_id", userID).Int("shift_count", len(shifts)).Msg("Successfully retrieved all shifts")
    return c.Status(http.StatusOK).JSON(models.Response{
        Success: true, Message: "Shifts retrieved successfully", Data: shifts,
    })
}