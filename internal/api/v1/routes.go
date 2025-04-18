package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger" // Middleware logger bawaan Fiber
	"github.com/rakaarfi/attendance-system-be/internal/api/v1/handlers"
	"github.com/rakaarfi/attendance-system-be/internal/middleware"
)

func SetupRoutes(app *fiber.App, authHandler *handlers.AuthHandler, adminHandler *handlers.AdminHandler, userHandler *handlers.UserHandler) {

	// Middleware global
	app.Use(logger.New()) // Log setiap request

	// Grouping API v1
	api := app.Group("/api/v1")

	// Rute Autentikasi (Publik)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)

	// Rute Admin (Perlu Login & Role Admin)
	admin := api.Group("/admin", middleware.Protected(), middleware.Authorize("Admin")) // Terapkan middleware auth & authorize
	//--shifts--
	admin.Post("/shifts", adminHandler.CreateShift)
	admin.Get("/shifts", adminHandler.GetAllShifts)
	admin.Get("/shifts/:shiftId", adminHandler.GetShiftByID) // Gunakan param
	admin.Put("/shifts/:shiftId", adminHandler.UpdateShift)
	admin.Delete("/shifts/:shiftId", adminHandler.DeleteShift)
	//--schedules--
	admin.Post("/schedules", adminHandler.CreateSchedule)
	admin.Get("/schedules", adminHandler.GetAllSchedules)
	admin.Put("/schedules/:scheduleId", adminHandler.UpdateSchedule)
	admin.Delete("/schedules/:scheduleId", adminHandler.DeleteSchedule)
	//--attendance--
	admin.Get("/attendance/report", adminHandler.GetAttendanceReport) // Laporan semua user
	//--users--
	admin.Get("/users", adminHandler.GetAllUsers)
	admin.Get("/users/:userId", adminHandler.GetUserByID)
	admin.Put("/users/:userId", adminHandler.UpdateUser)
	admin.Delete("/users/:userId", adminHandler.DeleteUser)
	admin.Get("/users/:userId/schedules", adminHandler.GetUserSchedules)
	admin.Get("/users/:userId/attendance", adminHandler.GetUserAttendance)
	// ... tambahkan rute admin lainnya (manage user, view all schedules, dll)

	// Rute User (Perlu Login, Role Employee atau Admin)
	user := api.Group("/user", middleware.Protected(), middleware.Authorize("Employee", "Admin")) // Employee & Admin bisa akses ini
	user.Post("/attendance/checkin", userHandler.CheckIn)
	user.Post("/attendance/checkout", userHandler.CheckOut)
	user.Get("/attendance/my", userHandler.GetMyAttendance) // Laporan user sendiri
	user.Get("/schedules/my", userHandler.GetMySchedules)
	user.Put("/profile", userHandler.UpdateMyProfile)
	user.Put("/password", userHandler.UpdateMyPassword)
	// ... tambahkan rute user lainnya (lihat jadwal sendiri, profil, dll)

	// Rute Health Check (Publik)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "UP"})
	})
}
