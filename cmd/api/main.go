package main

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/rakaarfi/attendance-system-be/configs"
	v1 "github.com/rakaarfi/attendance-system-be/internal/api/v1"
	"github.com/rakaarfi/attendance-system-be/internal/api/v1/handlers"
	"github.com/rakaarfi/attendance-system-be/internal/database"
	applogger "github.com/rakaarfi/attendance-system-be/internal/logger"
	appmiddleware "github.com/rakaarfi/attendance-system-be/internal/middleware"
	"github.com/rakaarfi/attendance-system-be/internal/repository"
	zlog "github.com/rs/zerolog/log"
)

func main() {
	// 0. Load Konfigurasi
	configs.LoadConfig()
	fmt.Println("Configuration loaded (pre-logger)")

	// 1. Setup Logger (Panggil paling awal dan tangkap closer)
	logCloser := applogger.SetupLogger()
	// Defer penutupan file logger jika ada
	if logCloser != nil {
		defer func() {
			zlog.Info().Msg("Closing log file...")
			if err := logCloser.Close(); err != nil {
				// Log error penutupan ke Stderr karena logger file mungkin sudah tidak bisa diakses
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to close log file: %v\n", err)
			}
		}()
	}
	// Sekarang baru log bahwa config sudah di-load
	zlog.Info().Msg("Configuration loaded")

	// 2. Koneksi Database
	dbPool, err := database.NewPgxPool()
	if err != nil {
		zlog.Fatal().Err(err).Msg("Could not connect to the database")
	}
	// Defer penutupan DB Pool *setelah* defer penutupan logger
	defer dbPool.Close()
	zlog.Info().Msg("Database connection pool established")

	// 3. Inisialisasi Repositories
	userRepo := repository.NewUserRepository(dbPool)
	roleRepo := repository.NewRoleRepository(dbPool)
	shiftRepo := repository.NewShiftRepository(dbPool)
	scheduleRepo := repository.NewScheduleRepository(dbPool)
	attendanceRepo := repository.NewAttendanceRepository(dbPool)

	// 4. Inisialisasi Handlers
	authHandler := handlers.NewAuthHandler(userRepo, roleRepo)
	// Inisialisasi AdminHandler dan UserHandler dengan dependensi yang benar
	adminHandler := handlers.NewAdminHandler(shiftRepo, scheduleRepo, attendanceRepo, userRepo, roleRepo)
	userHandler := handlers.NewUserHandler(attendanceRepo, scheduleRepo, userRepo)

	// 5. Setup Fiber App
	app := fiber.New(fiber.Config{
		ErrorHandler: handlers.ErrorHandler,
	})

	// 6. Setup Rute (Tambahkan middleware logger)
	appmiddleware.SetupGlobalMiddleware(app) // Panggil fungsi setup middleware
	v1.SetupRoutes(app, authHandler, adminHandler, userHandler)

	// 7. Start Server
	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "3000"
	}

	zlog.Info().Msgf("Server is starting on port %s...", appPort)
	startErr := app.Listen(fmt.Sprintf(":%s", appPort))
	if startErr != nil {
		zlog.Fatal().Err(startErr).Msg("Failed to start server")
	}
}
