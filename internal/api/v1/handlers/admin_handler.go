package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings" // Import strings
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	"github.com/rakaarfi/attendance-system-be/internal/repository"
	"github.com/rakaarfi/attendance-system-be/internal/utils"
	zlog "github.com/rs/zerolog/log"
)

type AdminHandler struct {
	ShiftRepo      repository.ShiftRepository
	ScheduleRepo   repository.ScheduleRepository
	AttendanceRepo repository.AttendanceRepository
	UserRepo       repository.UserRepository
	RoleRepo       repository.RoleRepository
	Validate       *validator.Validate
}

func NewAdminHandler(
	shiftRepo repository.ShiftRepository,
	scheduleRepo repository.ScheduleRepository,
	attRepo repository.AttendanceRepository,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
) *AdminHandler {
	return &AdminHandler{
		ShiftRepo:      shiftRepo,
		ScheduleRepo:   scheduleRepo,
		AttendanceRepo: attRepo,
		UserRepo:       userRepo,
		RoleRepo:       roleRepo,
		Validate:       validator.New(),
	}
}

func parseAdminDateQueryParams(c *fiber.Ctx) (startDate time.Time, endDate time.Time, err error) {
	now := time.Now()
	// Default rentang: Awal bulan ini sampai akhir hari ini
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" {
		startDate, err = time.Parse(defaultDateFormat, startDateStr)
		if err != nil {
			zlog.Warn().Err(err).Str("start_date_query", startDateStr).Msg("Invalid start_date format, using default")
			startDate = startOfMonth // Fallback
			err = nil                // Reset error agar tidak stop proses
		} else {
			// Set ke awal hari
			startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
		}
	} else {
		startDate = startOfMonth // Default jika tidak ada query param
	}

	if endDateStr != "" {
		endDate, err = time.Parse(defaultDateFormat, endDateStr)
		if err != nil {
			zlog.Warn().Err(err).Str("end_date_query", endDateStr).Msg("Invalid end_date format, using default")
			endDate = todayEnd // Fallback
			err = nil          // Reset error
		} else {
			// Set ke akhir hari
			endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, endDate.Location())
		}
	} else {
		endDate = todayEnd // Default jika tidak ada query param
	}

	if endDate.Before(startDate) {
		err = errors.New("end_date cannot be before start_date")
		return
	}
	return startDate, endDate, nil
}

// --- Shift Management ---

func (h *AdminHandler) CreateShift(c *fiber.Ctx) error {
	input := new(models.Shift)

	if err := c.BodyParser(input); err != nil {
		zlog.Error().Err(err).Msg("Error parsing create shift input")
		// Pastikan Data ada di error response
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false,
			Message: "Invalid request body",
			Data:    err.Error(), // Sertakan error di Data
		})
	}

	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Msg("Validation failed during shift creation")
		// Pastikan Data ada di error response
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false,
			Message: "Validation failed",
			Data:    err.Error(), // Sertakan error di Data
		})
	}

	zlog.Debug().Msg("Attempting to create shift in DB")
	shiftID, err := h.ShiftRepo.CreateShift(context.Background(), input)
	if err != nil {
		// Handle specific errors like invalid time format
		// Pesan error ini harusnya datang dari repo
		if err.Error() == "invalid time format, use HH:MM:SS" {
			zlog.Warn().Err(err).Msg("Invalid time format during shift creation")
			// Pastikan Data ada di error response
			return c.Status(fiber.StatusBadRequest).JSON(models.Response{
				Success: false,
				Message: "Invalid time format, use HH:MM:SS", // Pesan bersih
				Data:    err.Error(),                         // Sertakan error asli di Data
			})
		}
		zlog.Error().Err(err).Msg("Error creating shift in DB")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false,
			Message: "Failed to create shift", // Pesan generik untuk 500
		})
	}

	zlog.Info().Int("shift_id", shiftID).Msg("Shift created successfully")
	return c.Status(http.StatusCreated).JSON(models.Response{ // Gunakan 201 Created
		Success: true,
		Message: "Shift created successfully",
		Data:    fiber.Map{"shift_id": shiftID},
	})
}

func (h *AdminHandler) GetAllShifts(c *fiber.Ctx) error {
	shifts, err := h.ShiftRepo.GetAllShifts(context.Background())
	if err != nil {
		zlog.Error().Err(err).Msg("Error getting all shifts")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve shifts",
		})
	}

	zlog.Info().Msg("Shifts retrieved successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Shifts retrieved successfully", Data: shifts,
	})
}

func (h *AdminHandler) GetShiftByID(c *fiber.Ctx) error {
	idStr := c.Params("shiftId")
	shiftID, err := strconv.Atoi(idStr)
	if err != nil {
		zlog.Warn().Err(err).Str("shiftId_param", idStr).Msg("Invalid Shift ID parameter")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid Shift ID parameter", Data: err.Error(),
		})
	}

	shift, err := h.ShiftRepo.GetShiftByID(context.Background(), shiftID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Info().Int("shift_id", shiftID).Msg("Shift with ID not found")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("Shift with ID %d not found", shiftID),
			})
		}
		zlog.Error().Err(err).Int("shift_id", shiftID).Msg("Error getting shift by ID")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve shift",
		})
	}

	zlog.Info().Int("shift_id", shiftID).Msg("Shift retrieved successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Shift retrieved successfully", Data: shift,
	})
}

func (h *AdminHandler) UpdateShift(c *fiber.Ctx) error {
	idStr := c.Params("shiftId")
	shiftID, err := strconv.Atoi(idStr)
	if err != nil {
		zlog.Warn().Err(err).Str("shiftId_param", idStr).Msg("Invalid Shift ID parameter")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid Shift ID parameter", Data: err.Error(),
		})
	}

	input := new(models.Shift)
	if err := c.BodyParser(input); err != nil {
		zlog.Warn().Err(err).Msg("Invalid request body for update shift")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid request body", Data: err.Error(),
		})
	}

	input.ID = shiftID

	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Int("shift_id", shiftID).Msg("Validation failed during shift update")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	err = h.ShiftRepo.UpdateShift(context.Background(), input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Info().Int("shift_id", shiftID).Msg("Shift with ID not found for update")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("Shift with ID %d not found", shiftID),
			})
		}
		// Asumsi repo UpdateShift juga bisa mengembalikan error format waktu
		if err.Error() == "invalid time format, use HH:MM:SS" {
			zlog.Warn().Err(err).Int("shift_id", shiftID).Msg("Invalid time format during shift update")
			return c.Status(fiber.StatusBadRequest).JSON(models.Response{
				Success: false, Message: "Invalid time format, use HH:MM:SS", Data: err.Error(),
			})
		}
		zlog.Error().Err(err).Int("shift_id", shiftID).Msg("Error updating shift")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to update shift",
		})
	}

	zlog.Info().Int("shift_id", shiftID).Msg("Shift updated successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Shift updated successfully",
	})
}

func (h *AdminHandler) DeleteShift(c *fiber.Ctx) error {
	idStr := c.Params("shiftId")
	shiftID, err := strconv.Atoi(idStr)
	if err != nil {
		zlog.Warn().Err(err).Str("shiftId_param", idStr).Msg("Invalid Shift ID parameter")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid Shift ID parameter", Data: err.Error(),
		})
	}

	err = h.ShiftRepo.DeleteShift(context.Background(), shiftID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Info().Int("shift_id", shiftID).Msg("Shift with ID not found for delete")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("Shift with ID %d not found", shiftID),
			})
		}
		if err.Error() == "cannot delete shift: it is still referenced by user schedules" {
			zlog.Warn().Err(err).Int("shift_id", shiftID).Msg("Cannot delete shift due to FK constraint")
			return c.Status(fiber.StatusConflict).JSON(models.Response{
				Success: false, Message: err.Error(),
			})
		}
		zlog.Error().Err(err).Int("shift_id", shiftID).Msg("Error deleting shift")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to delete shift",
		})
	}

	zlog.Info().Int("shift_id", shiftID).Msg("Shift deleted successfully")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "Shift deleted successfully",
	})
}

// --- Schedule Management ---

func (h *AdminHandler) CreateSchedule(c *fiber.Ctx) error {
	input := new(models.UserSchedule)

	if err := c.BodyParser(input); err != nil {
		zlog.Warn().Err(err).Msg("Invalid request body for create schedule")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid request body", Data: err.Error(),
		})
	}

	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Msg("Validation failed during schedule creation")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	// Optional: Validasi user/shift ID di sini (jika di-enable, tambahkan mock di test)
	// _, errUser := h.UserRepo.GetUserByID(context.Background(), input.UserID)
	// _, errShift := h.ShiftRepo.GetShiftByID(context.Background(), input.ShiftID)
	// if errUser != nil || errShift != nil {
	//     zlog.Warn().Msgf("Validation failed for user/shift ID in create schedule: UserID=%d, ShiftID=%d, ErrUser=%v, ErrShift=%v", input.UserID, input.ShiftID, errUser, errShift)
	// 	return c.Status(fiber.StatusBadRequest).JSON(models.Response{
	// 			Success: false, Message: "Invalid User ID or Shift ID provided",
	// 		})
	// }

	scheduleID, err := h.ScheduleRepo.CreateSchedule(context.Background(), input)
	if err != nil {
		errMsg := "Failed to create schedule"
		status := fiber.StatusInternalServerError
		data := interface{}(nil) // Default data nil

		// Cek error spesifik dari repo
		// Pesan error dari repo mungkin seperti: "user %d already has a schedule on %s"
		if strings.Contains(err.Error(), "already has a schedule on") {
			errMsg = err.Error()
			status = fiber.StatusConflict
			data = err.Error() // Kirim error asli di data
		} else if strings.Contains(err.Error(), "invalid user_id") || strings.Contains(err.Error(), "invalid shift_id") {
			// Pesan error dari repo mungkin seperti: "invalid user_id (2) or shift_id (999)"
			errMsg = err.Error()
			status = fiber.StatusBadRequest
			data = err.Error()
		} else if strings.Contains(err.Error(), "invalid date format") {
			// Pesan error dari repo mungkin seperti: "invalid date format for schedule, use YYYY-MM-DD: ..."
			errMsg = "Invalid date format, use YYYY-MM-DD" // Pesan bersih
			status = fiber.StatusBadRequest
			data = err.Error() // Kirim error asli di data
		} else {
			zlog.Error().Err(err).Int("user_id", input.UserID).Int("shift_id", input.ShiftID).Msg("Error creating schedule")
		}
		return c.Status(status).JSON(models.Response{
			Success: false, Message: errMsg, Data: data, // Sertakan Data
		})
	}

	zlog.Info().Int("scheduleId", scheduleID).Int("user_id", input.UserID).Int("shift_id", input.ShiftID).Msg("Schedule created successfully")
	return c.Status(http.StatusCreated).JSON(models.Response{ // Gunakan 201 Created
		Success: true, Message: "Schedule created successfully", Data: fiber.Map{"scheduleId": scheduleID},
	})
}

// GetUserSchedules mengambil daftar jadwal untuk user tertentu berdasarkan ID
func (h *AdminHandler) GetUserSchedules(c *fiber.Ctx) error {
	// 1. Parse User ID
	targetUserIdStr := c.Params("userId")
	targetUserId, err := strconv.Atoi(targetUserIdStr)
	if err != nil {
		zlog.Warn().Err(err).Str("param", targetUserIdStr).Msg("Invalid User ID parameter for getting schedules")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid User ID parameter",
		})
	}

	// 2. Parse Tanggal
	startDate, endDate, dateErr := parseAdminDateQueryParams(c)
	if dateErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: dateErr.Error()})
	}

	// 3. Verifikasi User ID (opsional)
	_, errUser := h.UserRepo.GetUserByID(context.Background(), targetUserId)
	if errUser != nil { /* ... handle user not found (404) atau error lain (500) ... */
		if errors.Is(errUser, pgx.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(models.Response{Success: false, Message: fmt.Sprintf("User with ID %d not found", targetUserId)})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{Success: false, Message: "Failed to verify target user"})
	}

	// 4. Parse Pagination
	pagination := utils.ParsePaginationParams(c)

	// 5. Panggil Repository (Asumsi repo sudah diupdate untuk pagination)
	schedules, totalCount, err := h.ScheduleRepo.GetSchedulesByUser(context.Background(), targetUserId, startDate, endDate, pagination.Page, pagination.Limit)
	if err != nil {
		zlog.Error().Err(err).Int("target_user_id", targetUserId).Msg("Failed to get user schedules from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{Success: false, Message: "Failed to retrieve schedules for the user"})
	}

	// 6. Bangun Metadata dan Response
	meta := utils.BuildPaginationMeta(totalCount, pagination.Limit, pagination.Page)
	// response := utils.NewPaginatedResponse("User schedules retrieved successfully", schedules, meta)
	// Versi non-generic:
	response := struct {
		Success bool                  `json:"success"`
		Message string                `json:"message"`
		Data    []models.UserSchedule `json:"data"`
		Meta    utils.PaginationMeta  `json:"meta"`
	}{
		Success: true,
		Message: "User schedules retrieved successfully",
		Data:    schedules,
		Meta:    meta,
	}

	adminUserId, _ := utils.ExtractUserIDFromJWT(c) // Untuk log
	zlog.Info().
		Int("admin_id", adminUserId).
		Int("target_user_id", targetUserId).
		Int("schedule_count", len(schedules)).
		Time("start_date", startDate).
		Time("end_date", endDate).
		Msg("Admin successfully retrieved schedules for user")

	return c.Status(http.StatusOK).JSON(response)
}

func (h *AdminHandler) GetAllSchedules(c *fiber.Ctx) error {
	// 1. Parse Tanggal
	startDate, endDate, dateErr := parseAdminDateQueryParams(c)
	if dateErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: dateErr.Error()})
	}
	
	// 2. Parse Pagination
	pagination := utils.ParsePaginationParams(c)
	
	// 3. Panggil Repository (Asumsi repo sudah diupdate)
	schedules, totalCount, err := h.ScheduleRepo.GetSchedulesByDateRangeForAllUsers(context.Background(), startDate, endDate, pagination.Page, pagination.Limit)
	if err != nil {
		zlog.Error().Err(err).Msg("Failed to get all schedules from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{Success: false, Message: "Failed to retrieve schedules"})
	}

	// 4. Bangun Metadata dan Response
	meta := utils.BuildPaginationMeta(totalCount, pagination.Limit, pagination.Page)
	// response := utils.NewPaginatedResponse("Schedules retrieved successfully", schedules, meta)
	// Versi non-generic:
	response := struct {
		Success bool                  `json:"success"`
		Message string                `json:"message"`
		Data    []models.UserSchedule `json:"data"`
		Meta    utils.PaginationMeta  `json:"meta"`
	}{
		Success: true,
		Message: "Schedules retrieved successfully",
		Data:    schedules,
		Meta:    meta,
	}

	adminUserId, _ := utils.ExtractUserIDFromJWT(c) // Untuk log
	zlog.Info().
		Int("admin_id", adminUserId).
		Time("start_date", startDate).
		Time("end_date", endDate).
		Int("page", pagination.Page).
		Int("limit", pagination.Limit).
		Int("schedule_count", len(schedules)).
		Msg("Schedules retrieved successfully")

	return c.Status(http.StatusOK).JSON(response)
}

func (h *AdminHandler) UpdateSchedule(c *fiber.Ctx) error {
	scheduleIDStr := c.Params("scheduleId") // Sesuaikan nama param
	scheduleID, err := strconv.Atoi(scheduleIDStr)
	if err != nil {
		zlog.Warn().Err(err).Msg("Invalid schedule ID")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid schedule ID",
		})
	}

	input := new(models.UserSchedule)
	if err := c.BodyParser(input); err != nil {
		zlog.Warn().Err(err).Msg("Invalid request body for update schedule")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid request body", Data: err.Error(),
		})
	}

	// --- Validasi Input Struct ---
	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Msg("Update schedule validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	// --- Validasi User ID dan Shift ID ---
	_, errUser := h.UserRepo.GetUserByID(context.Background(), input.UserID)
	_, errShift := h.ShiftRepo.GetShiftByID(context.Background(), input.ShiftID)
	if errUser != nil || errShift != nil {
		zlog.Warn().Msgf("Validation failed for user/shift ID in update schedule: UserID=%d, ShiftID=%d, ErrUser=%v, ErrShift=%v", input.UserID, input.ShiftID, errUser, errShift)
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid User ID or Shift ID provided",
		})
	}

	input.ID = scheduleID // Set ID dari parameter URL
	err = h.ScheduleRepo.UpdateSchedule(context.Background(), input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Warn().Int("schedule_id", scheduleID).Msg("Attempted to update non-existent schedule")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("Schedule with ID %d not found", scheduleID),
			})
		}
		if strings.Contains(err.Error(), "already has a schedule on") { // Cek error unique constraint
			zlog.Warn().Err(err).Int("schedule_id", scheduleID).Msg("Unique constraint violation during schedule update")
			return c.Status(fiber.StatusConflict).JSON(models.Response{Success: false, Message: err.Error()})
		}
		if strings.Contains(err.Error(), "invalid user_id") || strings.Contains(err.Error(), "invalid shift_id") { // Cek error FK
			zlog.Warn().Err(err).Int("schedule_id", scheduleID).Msg("Foreign key violation during schedule update")
			return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: err.Error()})
		}
		if strings.Contains(err.Error(), "invalid date format") { // Cek error format tanggal
			zlog.Warn().Err(err).Int("schedule_id", scheduleID).Msg("Invalid date format during schedule update")
			return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: "Invalid date format, use YYYY-MM-DD"})
		}

		// Error fallback
		zlog.Error().Err(err).Int("schedule_id", scheduleID).Msg("Error updating schedule")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to update schedule",
		})
	}

	zlog.Info().Int("scheduleId", scheduleID).Msg("Schedule updated successfully")
	return c.Status(fiber.StatusOK).JSON(models.Response{
		Success: true, Message: "Schedule updated successfully",
	})
}

func (h *AdminHandler) DeleteSchedule(c *fiber.Ctx) error {
	scheduleIDStr := c.Params("scheduleId") // Sesuaikan nama param
	scheduleID, err := strconv.Atoi(scheduleIDStr)
	if err != nil {
		zlog.Warn().Err(err).Msg("Invalid schedule ID")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid schedule ID",
		})
	}

	err = h.ScheduleRepo.DeleteSchedule(context.Background(), scheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Warn().Int("schedule_id", scheduleID).Msg("Attempted to delete non-existent schedule")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("Schedule with ID %d not found", scheduleID),
			})
		}

		zlog.Error().Err(err).Int("schedule_id", scheduleID).Msg("Error deleting schedule")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to delete schedule",
		})
	}

	zlog.Info().Int("schedule_id", scheduleID).Msg("Schedule deleted successfully")
	return c.Status(fiber.StatusOK).JSON(models.Response{
		Success: true, Message: "Schedule deleted successfully",
	})
}

// --- Attendance Reporting ---

const defaultDateFormat = "2006-01-02"

// parseDateQueryParam parses YYYY-MM-DD query param or returns default
func parseDateQueryParam(c *fiber.Ctx, paramName string, defaultValue time.Time) time.Time {
	dateStr := c.Query(paramName)
	if dateStr == "" {
		zlog.Debug().Str("param", paramName).Msg("Query param empty, using default value")
		return defaultValue
	}
	t, err := time.Parse(defaultDateFormat, dateStr)
	if err != nil {
		zlog.Warn().Err(err).Str("param", paramName).Str("value", dateStr).Msg("Invalid date format in query param, using default value")
		return defaultValue
	}
	localLoc, _ := time.LoadLocation("Local")
	parsedDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, localLoc)
	zlog.Debug().Str("param", paramName).Time("parsed_date", parsedDate).Msg("Date query param parsed successfully")
	return parsedDate

}

// GetUserAttendance mengambil catatan kehadiran untuk user tertentu berdasarkan ID
func (h *AdminHandler) GetUserAttendance(c *fiber.Ctx) error {
	// 1. Dapatkan ID user target
	targetUserIdStr := c.Params("userId")
	targetUserId, err := strconv.Atoi(targetUserIdStr)
	if err != nil {
		zlog.Warn().Err(err).Str("param", targetUserIdStr).Msg("Invalid User ID parameter for getting attendance")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid User ID parameter",
		})
	}

	// 2. Parse Tanggal
	startDate, endDate, dateErr := parseAdminDateQueryParams(c)
	if dateErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: dateErr.Error()})
	}

	// 3. (Opsional tapi bagus) Verifikasi User ID target
	_, errUser := h.UserRepo.GetUserByID(context.Background(), targetUserId)
	if errUser != nil { /* ... handle user not found (404) atau error lain (500) ... */
		if errors.Is(errUser, pgx.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(models.Response{Success: false, Message: fmt.Sprintf("User with ID %d not found", targetUserId)})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{Success: false, Message: "Failed to verify target user"})
	}

	// 4. Parse Pagination
	pagination := utils.ParsePaginationParams(c)

	// 5. Panggil Repository
	attendances, totalCount, err := h.AttendanceRepo.GetAttendancesByUser(context.Background(), targetUserId, startDate, endDate, pagination.Page, pagination.Limit)
	if err != nil {
		zlog.Error().Err(err).Int("target_user_id", targetUserId).Msg("Failed to get user attendance from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve attendance records for the user",
		})
	}

	// 6. Bangun Metadata dan Response
	meta := utils.BuildPaginationMeta(totalCount, pagination.Limit, pagination.Page)
	// response := utils.NewPaginatedResponse("User attendance records retrieved successfully", attendances, meta)
	// Versi non-generic:
	response := struct {
		Success bool                 `json:"success"`
		Message string               `json:"message"`
		Data    []models.Attendance  `json:"data"`
		Meta    utils.PaginationMeta `json:"meta"`
	}{
		Success: true,
		Message: "User attendance records retrieved successfully",
		Data:    attendances,
		Meta:    meta,
	}

	adminUserId, _ := utils.ExtractUserIDFromJWT(c) // Untuk log
	zlog.Info().
		Int("admin_id", adminUserId).
		Int("target_user_id", targetUserId).
		Int("page", pagination.Page).
		Int("limit", pagination.Limit).
		Int("returned_count", len(attendances)).
		Int("total_count", totalCount).
		Msg("Admin successfully retrieved paginated attendance for user")

	return c.Status(http.StatusOK).JSON(response)
}

func (h *AdminHandler) GetAttendanceReport(c *fiber.Ctx) error {
	// 1. Parse Tanggal
	startDate, endDate, dateErr := parseAdminDateQueryParams(c)
	if dateErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: dateErr.Error()})
	}

	// 2. Parse Pagination
	pagination := utils.ParsePaginationParams(c)

	// 3. Panggil Repository
	attendances, totalCount, err := h.AttendanceRepo.GetAllAttendances(context.Background(), startDate, endDate, pagination.Page, pagination.Limit)
	if err != nil {
		zlog.Error().Err(err).Msg("Failed to get attendance report from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve attendance report",
		})
	}

	// 4. Bangun Metadata dan Response
	meta := utils.BuildPaginationMeta(totalCount, pagination.Limit, pagination.Page)
	// Gunakan tipe spesifik jika tidak pakai generic, atau gunakan generic helper
	// response := utils.NewPaginatedResponse("Attendance report retrieved successfully", attendances, meta)
	// Versi non-generic:
	response := struct {
		Success bool                 `json:"success"`
		Message string               `json:"message"`
		Data    []models.Attendance  `json:"data"`
		Meta    utils.PaginationMeta `json:"meta"`
	}{
		Success: true,
		Message: "Attendance report retrieved successfully",
		Data:    attendances,
		Meta:    meta,
	}

	adminUserId, _ := utils.ExtractUserIDFromJWT(c) // Untuk log
	zlog.Info().
		Int("admin_id", adminUserId).
		Int("page", pagination.Page).
		Int("limit", pagination.Limit).
		Int("returned_count", len(attendances)).
		Int("total_count", totalCount).
		Msg("Successfully retrieved paginated attendance report")

	return c.Status(http.StatusOK).JSON(response)
}

// --- User Management --
func (h *AdminHandler) GetAllUsers(c *fiber.Ctx) error {
	// --- 1. Baca dan Validasi Parameter Pagination ---
	page, err := strconv.Atoi(c.Query("page", "1")) // Default page 1
	if err != nil || page < 1 {
		zlog.Warn().Str("page_query", c.Query("page", "1")).Msg("Invalid page query parameter, using default 1")
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "10")) // Default limit 10
	if err != nil || limit < 1 {
		zlog.Warn().Str("limit_query", c.Query("limit", "10")).Msg("Invalid limit query parameter, using default 10")
		limit = 10
	}
	// Opsional: Batasi limit maksimum
	const maxLimit = 100
	if limit > maxLimit {
		zlog.Warn().Int("requested_limit", limit).Int("max_limit", maxLimit).Msg("Requested limit exceeds maximum, capping")
		limit = maxLimit
	}

	// --- 2. Panggil Repository dengan Parameter Pagination ---
	users, totalCount, err := h.UserRepo.GetAllUsers(context.Background(), page, limit)
	if err != nil {
		// Error sudah di-log di repo, tapi log di handler juga baik untuk konteks request
		zlog.Error().Err(err).Int("page", page).Int("limit", limit).Msg("Failed to get users from repository (paginated)")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve users",
		})
	}

	// --- 3. Siapkan Response dengan Metadata ---
	totalPages := 0
	if totalCount > 0 && limit > 0 { // Hindari pembagian dengan nol
		totalPages = int(math.Ceil(float64(totalCount) / float64(limit)))
	}

	// Buat struktur data response baru yang menyertakan metadata
	paginatedResponse := struct {
		Success bool          `json:"success"`
		Message string        `json:"message"`
		Data    []models.User `json:"data"`
		Meta    struct {
			CurrentPage int `json:"current_page"`
			PerPage     int `json:"per_page"`
			TotalItems  int `json:"total_items"`
			TotalPages  int `json:"total_pages"`
		} `json:"meta"`
	}{
		Success: true,
		Message: "Users retrieved successfully",
		Data:    users, // Data user untuk halaman ini
		Meta: struct {
			CurrentPage int `json:"current_page"`
			PerPage     int `json:"per_page"`
			TotalItems  int `json:"total_items"`
			TotalPages  int `json:"total_pages"`
		}{
			CurrentPage: page,
			PerPage:     limit,
			TotalItems:  totalCount,
			TotalPages:  totalPages,
		},
	}

	zlog.Info().
		Int("page", page).
		Int("limit", limit).
		Int("returned_count", len(users)).
		Int("total_count", totalCount).
		Msg("Successfully retrieved paginated users for admin request")

		// Kirim response terstruktur
	return c.Status(http.StatusOK).JSON(paginatedResponse)
}

func (h *AdminHandler) GetUserByID(c *fiber.Ctx) error {
	userIdStr := c.Params("userId")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		zlog.Warn().Err(err).Str("param", userIdStr).Msg("Invalid User ID parameter")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid User ID parameter",
		})
	}

	adminUserId, _ := utils.ExtractUserIDFromJWT(c) // Abaikan error sementara jika hanya untuk log

	user, err := h.UserRepo.GetUserByID(context.Background(), userId)
	if err != nil {
		// --- CEK NOT FOUND ---
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Warn().Int("requested_user_id", userId).Msg("Admin requested non-existent user")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("User with ID %d not found", userId),
			})
		}
		zlog.Error().Err(err).Int("user_id", userId).Msg("Failed to get user from repository")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to retrieve user",
		})
	}
	// Logging sukses
	zlog.Info().Int("user_id", userId).Int("admin_id", adminUserId).Msg("Successfully retrieved user for admin request")
	// Logging sukses
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: "User retrieved successfully", Data: user,
	})
}

func (h *AdminHandler) UpdateUser(c *fiber.Ctx) error {
	// 1. Dapatkan ID user target dari URL
	targetUserIdStr := c.Params("userId")
	targetUserId, err := strconv.Atoi(targetUserIdStr)
	if err != nil {
		zlog.Warn().Err(err).Str("param", targetUserIdStr).Msg("Invalid User ID parameter for update")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid User ID parameter",
		})
	}

	// 2. Dapatkan ID admin yang sedang login (opsional, tapi bisa berguna untuk log)
	adminUserId, _ := utils.ExtractUserIDFromJWT(c) // Abaikan error sementara jika hanya untuk log

	// 3. Parse & Validasi Input Body (Gunakan struct input baru)
	input := new(models.AdminUpdateUserInput) // <-- Gunakan input model baru
	if err := c.BodyParser(input); err != nil {
		zlog.Error().Err(err).Msg("Error parsing update user request body")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Failed to parse request body",
		})
	}

	// 4. Validasi data input menggunakan validator
	if err := h.Validate.Struct(input); err != nil {
		zlog.Warn().Err(err).Msg("Update user validation failed")
		// Berikan detail error validasi jika perlu (hati-hati info sensitif)
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Validation failed", Data: err.Error(),
		})
	}

	// 5. (Opsional tapi direkomendasikan) Validasi Role ID
	_, errRole := h.RoleRepo.GetRoleByID(context.Background(), input.RoleID)
	if errRole != nil {
		// Handle jika role ID tidak valid
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{Success: false, Message: "Invalid Role ID"})
	}

	// 6. Panggil repository untuk update user
	err = h.UserRepo.UpdateUserByID(context.Background(), targetUserId, input) // <-- Pass input model baru
	if err != nil {
		// Cek apakah error karena user tidak ditemukan
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Warn().Int("target_user_id", targetUserId).Msg("Attempted to update non-existent user")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("User with ID %d not found", targetUserId),
			})
		}
		// Cek apakah error karena unique constraint
		if strings.Contains(err.Error(), "already exists") {
			zlog.Warn().Err(err).Int("target_user_id", targetUserId).Msg("Unique constraint violation during user update by admin")
			return c.Status(fiber.StatusConflict).JSON(models.Response{ // 409 Conflict
				Success: false, Message: err.Error(),
			})
		}

		// Error lain saat update
		zlog.Error().Err(err).Int("target_user_id", targetUserId).Msg("Failed to update user by admin")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to update user",
		})
	}

	// 7. Kirim response sukses
	zlog.Info().Int("admin_id", adminUserId).Int("updated_user_id", targetUserId).Msg("Admin successfully updated user")
	// Pertimbangkan untuk mengembalikan data user yang sudah diupdate (ambil lagi dari DB)
	// atau cukup pesan sukses
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: fmt.Sprintf("User with ID %d updated successfully", targetUserId),
	})
}

func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	// 1. Dapatkan ID user yang akan dihapus dari parameter URL
	targetUserIdStr := c.Params("userId") // Sesuaikan nama param dengan route nanti
	targetUserId, err := strconv.Atoi(targetUserIdStr)
	if err != nil {
		zlog.Warn().Err(err).Str("param", targetUserIdStr).Msg("Invalid User ID parameter for deletion")
		return c.Status(fiber.StatusBadRequest).JSON(models.Response{
			Success: false, Message: "Invalid User ID parameter",
		})
	}

	// 2. Dapatkan ID admin yang sedang login dari JWT (PENTING: untuk mencegah hapus diri sendiri)
	adminUserId, err := utils.ExtractUserIDFromJWT(c)
	if err != nil {
		zlog.Error().Err(err).Msg("Failed to extract admin user ID from JWT")
		// Ini seharusnya tidak terjadi jika middleware auth bekerja, tapi handle untuk keamanan
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to identify requesting admin",
		})
	}

	// 3. Validasi: Admin tidak boleh menghapus dirinya sendiri
	if targetUserId == adminUserId {
		zlog.Warn().Int("admin_id", adminUserId).Msg("Admin attempted to delete themselves")
		return c.Status(fiber.StatusForbidden).JSON(models.Response{
			Success: false, Message: "Admin cannot delete their own account",
		})
	}

	// 4. Panggil repository untuk menghapus user
	err = h.UserRepo.DeleteUserByID(context.Background(), targetUserId)
	if err != nil {
		// Cek apakah error karena user tidak ditemukan
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Warn().Int("target_user_id", targetUserId).Msg("Attempted to delete non-existent user")
			return c.Status(fiber.StatusNotFound).JSON(models.Response{
				Success: false, Message: fmt.Sprintf("User with ID %d not found", targetUserId),
			})
		}
		// Error lain saat menghapus
		zlog.Error().Err(err).Int("target_user_id", targetUserId).Msg("Failed to delete user")
		return c.Status(fiber.StatusInternalServerError).JSON(models.Response{
			Success: false, Message: "Failed to delete user",
		})
	}

	// 5. Kirim response sukses
	zlog.Info().Int("admin_id", adminUserId).Int("deleted_user_id", targetUserId).Msg("Admin successfully deleted user")
	return c.Status(http.StatusOK).JSON(models.Response{
		Success: true, Message: fmt.Sprintf("User with ID %d deleted successfully", targetUserId),
	})
}
