// internal/repository/repository.go
package repository

import (
	"context"
	"time"

	"github.com/rakaarfi/attendance-system-be/internal/models"
)

// File ini mendefinisikan **interfaces** untuk Data Access Layer (DAL).
// Interface ini berfungsi sebagai **kontrak** yang menentukan operasi data apa saja
// yang harus bisa dilakukan oleh implementasi repository konkret (misal: *_repo.go).
// Penggunaan interface memungkinkan decoupling (pemisahan) antara lapisan handler/service
// dengan implementasi akses data spesifik (misal: PostgreSQL).

// UserRepository: Kontrak untuk operasi data User.
type UserRepository interface {
	CreateUser(ctx context.Context, user *models.RegisterUserInput, hashedPassword string) (int, error) // Buat user baru.
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)                       // Cari user by username (termasuk role).
	GetUserByID(ctx context.Context, id int) (*models.User, error)                                      // Cari user by ID (termasuk role).
	DeleteUserByID(ctx context.Context, id int) error                                                   // Hapus user by ID.
	GetAllUsers(ctx context.Context, page, limit int) ([]models.User, int, error)                       // Dapatkan semua user (paginated, termasuk role).
	UpdateUserByID(ctx context.Context, id int, input *models.AdminUpdateUserInput) error               // Update user by ID (oleh Admin).
	UpdateUserPassword(ctx context.Context, id int, hashedPassword string) error                        // Update password user by ID (dengan hash).
	UpdateUserProfile(ctx context.Context, id int, input *models.UpdateProfileInput) error              // Update profil user by ID (oleh user sendiri).
}

// ShiftRepository: Kontrak untuk operasi data Shift (definisi jam kerja).
type ShiftRepository interface {
	CreateShift(ctx context.Context, shift *models.Shift) (int, error) // Buat shift baru.
	GetShiftByID(ctx context.Context, id int) (*models.Shift, error)   // Cari shift by ID.
	GetAllShifts(ctx context.Context) ([]models.Shift, error)          // Dapatkan semua shift.
	UpdateShift(ctx context.Context, shift *models.Shift) error        // Update shift by ID.
	DeleteShift(ctx context.Context, id int) error                     // Hapus shift by ID (cek dependensi).
}

// ScheduleRepository: Kontrak untuk operasi data UserSchedule (penjadwalan).
type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule *models.UserSchedule) (int, error)                                                            // Buat jadwal baru.
	GetScheduleByUserAndDate(ctx context.Context, userID int, date time.Time) (*models.UserSchedule, error)                                    // Cari jadwal user pada tanggal tertentu.
	GetSchedulesByUser(ctx context.Context, userID int, startDate, endDate time.Time, page, limit int) ([]models.UserSchedule, int, error)     // Dapatkan jadwal user (paginated).
	GetSchedulesByDateRangeForAllUsers(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]models.UserSchedule, int, error) // Dapatkan semua jadwal (paginated).
	DeleteSchedule(ctx context.Context, id int) error                                                                                          // Hapus jadwal by ID.
	UpdateSchedule(ctx context.Context, schedule *models.UserSchedule) error                                                                   // Update jadwal by ID.
}

// AttendanceRepository: Kontrak untuk operasi data Attendance (log absensi).
type AttendanceRepository interface {
	CreateCheckIn(ctx context.Context, userID int, checkInTime time.Time, notes *string) (int, error)                                      // Catat check-in.
	GetLastAttendance(ctx context.Context, userID int) (*models.Attendance, error)                                                         // Dapatkan absensi terakhir user.
	UpdateCheckOut(ctx context.Context, attendanceID int, checkOutTime time.Time, notes *string) error                                     // Catat check-out pada absensi ID tertentu.
	GetAttendancesByUser(ctx context.Context, userID int, startDate, endDate time.Time, page, limit int) ([]models.Attendance, int, error) // Dapatkan absensi user (paginated).
	GetAllAttendances(ctx context.Context, startDate, endDate time.Time, page, limit int) ([]models.Attendance, int, error)                // Dapatkan semua absensi (paginated, termasuk user).
}

// RoleRepository: Kontrak untuk operasi data Role.
type RoleRepository interface {
	CreateRole(ctx context.Context, role *models.Role) (int, error) // Buat role baru.
	GetRoleByID(ctx context.Context, id int) (*models.Role, error)  // Cari role by ID.
	GetAllRoles(ctx context.Context) ([]models.Role, error)         // Dapatkan semua role.
	UpdateRole(ctx context.Context, role *models.Role) error        // Update role by ID.
	DeleteRole(ctx context.Context, id int) error                   // Hapus role by ID (cek dependensi user).
}
