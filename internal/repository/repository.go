package repository

import (
	"context"
	"time"

	"github.com/rakaarfi/attendance-system-be/internal/models"
)

// Interface umum bisa ditambahkan jika diperlukan

type UserRepository interface {
	CreateUser(ctx context.Context, user *models.RegisterUserInput, hashedPassword string) (int, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByID(ctx context.Context, id int) (*models.User, error)
	DeleteUserByID(ctx context.Context, id int) error
	GetAllUsers(ctx context.Context, page, limit int) (users []models.User, totalCount int, err error)
	UpdateUserByID(ctx context.Context, id int, user *models.AdminUpdateUserInput) error
	UpdateUserPassword(ctx context.Context, id int, newPassword string) error
	UpdateUserProfile(ctx context.Context, id int, input *models.UpdateProfileInput) error
}

type ShiftRepository interface {
	CreateShift(ctx context.Context, shift *models.Shift) (int, error)
	GetShiftByID(ctx context.Context, id int) (*models.Shift, error)
	GetAllShifts(ctx context.Context) ([]models.Shift, error)
	UpdateShift(ctx context.Context, shift *models.Shift) error
	DeleteShift(ctx context.Context, id int) error
}

type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule *models.UserSchedule) (int, error)
	GetScheduleByUserAndDate(ctx context.Context, userID int, date time.Time) (*models.UserSchedule, error)
	GetSchedulesByUser(ctx context.Context, userID int, startDate, endDate time.Time, page, limit int) (schedules []models.UserSchedule, totalCount int, err error)
	GetSchedulesByDateRangeForAllUsers(ctx context.Context, startDate, endDate time.Time, page, limit int) (schedules []models.UserSchedule, totalCount int, err error)
	DeleteSchedule(ctx context.Context, id int) error
	UpdateSchedule(ctx context.Context, schedule *models.UserSchedule) error
}

type AttendanceRepository interface {
	CreateCheckIn(ctx context.Context, userID int, checkInTime time.Time, notes *string) (int, error)
	GetLastAttendance(ctx context.Context, userID int) (*models.Attendance, error)
	UpdateCheckOut(ctx context.Context, attendanceID int, checkOutTime time.Time, notes *string) error
	GetAttendancesByUser(ctx context.Context, userID int, startDate, endDate time.Time, page, limit int) (attendances []models.Attendance, totalCount int, err error)
	GetAllAttendances(ctx context.Context, startDate, endDate time.Time, page, limit int) (attendances []models.Attendance, totalCount int, err error) // Report juga dipaginasi
}

type RoleRepository interface {
	GetRoleByID(ctx context.Context, id int) (*models.Role, error)
}
