package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rakaarfi/attendance-system-be/internal/api/v1/handlers" // Handler spesifik v1
	"github.com/rakaarfi/attendance-system-be/internal/middleware"      // Middleware aplikasi (Auth, dll)
)

func SetupRoutes(app *fiber.App, authHandler *handlers.AuthHandler, adminHandler *handlers.AdminHandler, userHandler *handlers.UserHandler) {
	// -------------------------------------------------------------------------
	// Grouping Rute API v1
	// -------------------------------------------------------------------------
	// Membuat grup rute dengan prefix /api/v1
	api := app.Group("/api/v1")

	// =========================================================================
	// Rute Autentikasi (Publik - Tidak Memerlukan Login)
	// =========================================================================
	// Grup untuk endpoint yang berkaitan dengan autentikasi (/api/v1/auth)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register) // Endpoint untuk registrasi user baru
	auth.Post("/login", authHandler.Login)       // Endpoint untuk login dan mendapatkan token JWT

	// =========================================================================
	// Rute Admin (Memerlukan Login & Role 'Admin')
	// =========================================================================
	// Grup untuk endpoint khusus Admin (/api/v1/admin)
	// Middleware .Protected() memastikan user sudah login (valid JWT)
	// Middleware .Authorize("Admin") memastikan user memiliki role 'Admin'
	admin := api.Group("/admin", middleware.Protected(), middleware.Authorize("Admin"))

	// --- Manajemen Shift ---
	admin.Post("/shifts", adminHandler.CreateShift)            // Membuat definisi shift baru
	admin.Get("/shifts", adminHandler.GetAllShifts)            // Mendapatkan semua definisi shift
	admin.Get("/shifts/:shiftId", adminHandler.GetShiftByID)   // Mendapatkan detail shift berdasarkan ID
	admin.Put("/shifts/:shiftId", adminHandler.UpdateShift)    // Memperbarui definisi shift
	admin.Delete("/shifts/:shiftId", adminHandler.DeleteShift) // Menghapus definisi shift

	// --- Manajemen Jadwal (Penugasan Shift ke User) ---
	admin.Post("/schedules", adminHandler.CreateSchedule)               // Membuat jadwal baru untuk user pada tanggal tertentu
	admin.Get("/schedules", adminHandler.GetAllSchedules)               // Mendapatkan semua jadwal (bisa difilter tanggal)
	admin.Put("/schedules/:scheduleId", adminHandler.UpdateSchedule)    // Memperbarui jadwal yang sudah ada
	admin.Delete("/schedules/:scheduleId", adminHandler.DeleteSchedule) // Menghapus jadwal

	// --- Laporan Kehadiran (Admin View) ---
	admin.Get("/attendance/report", adminHandler.GetAttendanceReport) // Mendapatkan laporan kehadiran semua user (bisa difilter tanggal)

	// --- Manajemen Pengguna (oleh Admin) ---
	admin.Get("/users", adminHandler.GetAllUsers)           // Mendapatkan daftar semua user (dengan pagination)
	admin.Get("/users/:userId", adminHandler.GetUserByID)   // Mendapatkan detail user berdasarkan ID
	admin.Put("/users/:userId", adminHandler.UpdateUser)    // Memperbarui data user (username, email, nama, role)
	admin.Delete("/users/:userId", adminHandler.DeleteUser) // Menghapus user

	// --- Endpoint Tambahan Terkait User Spesifik (oleh Admin) ---
	// Melihat jadwal spesifik untuk user tertentu
	admin.Get("/users/:userId/schedules", adminHandler.GetUserSchedules)
	// Melihat rekap absensi spesifik untuk user tertentu
	admin.Get("/users/:userId/attendance", adminHandler.GetUserAttendance)

	// --- Manajemen Role (oleh Admin) ---
	admin.Post("/roles", adminHandler.CreateRole)           // Membuat role baru
	admin.Get("/roles", adminHandler.GetAllRoles)           // Mendapatkan daftar semua role
	admin.Get("/roles/:roleId", adminHandler.GetRoleByID)   // Mendapatkan detail role berdasarkan ID
	admin.Put("/roles/:roleId", adminHandler.UpdateRole)    // Memperbarui role
	admin.Delete("/roles/:roleId", adminHandler.DeleteRole) // Menghapus role

	// =========================================================================
	// Rute Pengguna (Memerlukan Login - Role 'Employee' atau 'Admin')
	// =========================================================================
	// Grup untuk endpoint yang bisa diakses oleh pengguna yang sudah login (/api/v1/user)
	// .Authorize("Employee", "Admin") mengizinkan kedua role mengakses endpoint ini.
	// Jika hanya Employee: middleware.Authorize("Employee")
	user := api.Group("/user", middleware.Protected()) // Dihapus Authorize agar Admin juga bisa tes/akses jika perlu

	// --- Kehadiran (Absensi) ---
	user.Post("/attendance/checkin", userHandler.CheckIn)   // Melakukan check-in
	user.Post("/attendance/checkout", userHandler.CheckOut) // Melakukan check-out
	user.Get("/attendance/my", userHandler.GetMyAttendance) // Melihat riwayat kehadiran diri sendiri (bisa difilter tanggal)

	// --- Jadwal Pribadi ---
	user.Get("/schedules/my", userHandler.GetMySchedules) // Melihat jadwal shift diri sendiri (bisa difilter tanggal)

	// --- Manajemen Profil Pribadi ---
	user.Get("/profile", userHandler.GetMyProfile)      // Mendapatkan profil sendiri
	user.Put("/profile", userHandler.UpdateMyProfile)   // Memperbarui data profil diri sendiri (nama, email, username)
	user.Put("/password", userHandler.UpdateMyPassword) // Mengubah password diri sendiri

	// =========================================================================
	// Rute Lain-lain (Publik)
	// =========================================================================
	api.Get("/health", HealthCheck)

	// Endpoint untuk melihat semua shift
	api.Get("/shifts", userHandler.GetAllShifts)
}

// HealthCheck godoc
// @Summary Check Health
// @Description Public endpoint to verify that the API is running and responsive.
// @Tags Public
// @ID health-check
// @Produce json
// @Success 200 {object} map[string]string `json:"status"`
// @Router /health [get]
func HealthCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "UP"})
}
