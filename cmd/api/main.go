package main

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"                                                // Framework web Fiber
	"github.com/rakaarfi/attendance-system-be/configs"                           // Paket lokal untuk konfigurasi
	v1 "github.com/rakaarfi/attendance-system-be/internal/api/v1"                // Paket lokal untuk routing API v1
	"github.com/rakaarfi/attendance-system-be/internal/api/v1/handlers"          // Paket lokal untuk handler API v1
	"github.com/rakaarfi/attendance-system-be/internal/database"                 // Paket lokal untuk koneksi database
	applogger "github.com/rakaarfi/attendance-system-be/internal/logger"         // Paket lokal untuk setup logger (Zerolog)
	appmiddleware "github.com/rakaarfi/attendance-system-be/internal/middleware" // Paket lokal untuk middleware global
	"github.com/rakaarfi/attendance-system-be/internal/repository"               // Paket lokal untuk repository (akses data)
	zlog "github.com/rs/zerolog/log"                                             // Logger global Zerolog (aliased as zlog)

	// Import untuk Swagger/OpenAPI documentation
	_ "github.com/rakaarfi/attendance-system-be/docs" // Import side effect untuk registrasi docs Swagger yang digenerate (penting!)
	fiberSwagger "github.com/swaggo/fiber-swagger"    // Middleware Fiber untuk menyajikan Swagger UI
)

// --- Anotasi Global Swagger/OpenAPI ---
// Anotasi ini dibaca oleh 'swag init' untuk menghasilkan dokumentasi API.
// @title Sistem Absensi Pegawai API              // Judul API
// @version 1.0                                   // Versi API saat ini
// @description API backend untuk sistem absensi pegawai dengan role dan shift. // Deskripsi singkat API
// @termsOfService http://swagger.io/terms/       // Link ke Terms of Service (jika ada)

// @contact.name API Support                      // Nama kontak support
// @contact.url http://www.swagger.io/support     // URL kontak support
// @contact.email support@swagger.io              // Email kontak support

// @license.name Apache 2.0                       // Nama lisensi API
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html // URL lisensi API

// @host localhost:3000                           // Host dan port tempat API dapat diakses (sesuaikan untuk production)
// @BasePath /api/v1                              // Path dasar untuk semua rute API versi 1

// @securityDefinitions.apikey ApiKeyAuth         // Mendefinisikan skema keamanan bernama 'ApiKeyAuth'
// @in header                                     // Menentukan bahwa kunci keamanan dikirim via header HTTP
// @name Authorization                            // Nama header HTTP yang digunakan (untuk Bearer token)
// @description "Type 'Bearer YOUR_JWT_TOKEN' into the value field." // Petunjuk penggunaan di Swagger UI
// --- Akhir Anotasi Swagger ---

// main adalah fungsi entry point aplikasi Go.
func main() {
	// --- Langkah 0: Load Konfigurasi dari .env ---
	// Membaca file .env dan memuat variabelnya ke environment process.
	// Harus dijalankan *sebelum* komponen lain yang bergantung pada env vars (seperti logger, db).
	configs.LoadConfig()
	// Hindari logging sebelum logger siap. fmt.Println bisa digunakan jika benar-benar perlu.
	// fmt.Println("Configuration loaded (pre-logger)")

	// --- Langkah 1: Setup Logger (Zerolog) ---
	// Menginisialisasi logger global (Zerolog) berdasarkan konfigurasi env vars (LOG_LEVEL, dll.).
	// Mengembalikan io.Closer jika file logging diaktifkan.
	logCloser := applogger.SetupLogger()
	// Menjadwalkan penutupan file log (jika ada) saat fungsi main selesai.
	if logCloser != nil {
		defer func() {
			zlog.Info().Msg("Closing log file...") // Log bahwa kita mencoba menutup file
			if err := logCloser.Close(); err != nil {
				// Jika gagal menutup, log error ke Stderr karena logger file mungkin sudah tidak bisa diakses.
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to close log file: %v\n", err)
			}
		}()
	}
	// Log pertama menggunakan Zerolog setelah setup selesai.
	zlog.Info().Msg("Configuration loaded")

	// --- Langkah 2: Koneksi ke Database (PostgreSQL) ---
	// Membuat connection pool ke database PostgreSQL menggunakan konfigurasi dari env vars.
	dbPool, err := database.NewPgxPool()
	if err != nil {
		// Jika koneksi gagal, log error fatal dan hentikan aplikasi.
		zlog.Fatal().Err(err).Msg("Could not connect to the database")
	}
	// Menjadwalkan penutupan connection pool saat fungsi main selesai.
	// Defer ini diletakkan setelah defer logger agar error penutupan DB masih bisa di-log.
	defer dbPool.Close()
	zlog.Info().Msg("Database connection pool established")

	// --- Langkah 3: Inisialisasi Lapisan Repository ---
	// Membuat instance konkret dari setiap repository, menyuntikkan (injecting)
	// connection pool (dbPool) sebagai dependensi.
	userRepo := repository.NewUserRepository(dbPool)
	roleRepo := repository.NewRoleRepository(dbPool)
	shiftRepo := repository.NewShiftRepository(dbPool)
	scheduleRepo := repository.NewScheduleRepository(dbPool)
	attendanceRepo := repository.NewAttendanceRepository(dbPool)
	zlog.Info().Msg("Repositories initialized")

	// --- Langkah 4: Inisialisasi Lapisan Handler ---
	// Membuat instance konkret dari setiap handler, menyuntikkan repository
	// yang relevan sebagai dependensi.
	authHandler := handlers.NewAuthHandler(userRepo, roleRepo)
	adminHandler := handlers.NewAdminHandler(shiftRepo, scheduleRepo, attendanceRepo, userRepo, roleRepo)
	userHandler := handlers.NewUserHandler(attendanceRepo, scheduleRepo, userRepo, shiftRepo)
	zlog.Info().Msg("Handlers initialized")

	// --- Langkah 5: Setup Aplikasi Fiber ---
	// Membuat instance baru dari aplikasi web Fiber.
	// Mengkonfigurasi ErrorHandler global kustom dari paket handlers.
	app := fiber.New(fiber.Config{
		ErrorHandler: handlers.ErrorHandler,
	})
	zlog.Info().Msg("Fiber app initialized")

	// --- Langkah 6: Setup Middleware Global dan Rute ---
	// Mendaftarkan middleware global (seperti logger request, CORS, recover) ke aplikasi Fiber.
	appmiddleware.SetupGlobalMiddleware(app)

	// Mendaftarkan endpoint untuk Swagger UI.
	// Harus didaftarkan *sebelum* rute API utama jika prefix-nya sama atau tumpang tindih.
	// URL: http://<host>/swagger/index.html
	app.Get("/swagger/*", fiberSwagger.WrapHandler)
	zlog.Info().Msg("Swagger UI endpoint registered at /swagger/*")

	// Mendaftarkan semua rute API versi 1 (/api/v1/...) dengan menyuntikkan handler yang sesuai.
	v1.SetupRoutes(app, authHandler, adminHandler, userHandler)
	zlog.Info().Msg("API v1 routes registered")

	// --- Langkah 7: Start Server HTTP ---
	// Mendapatkan port dari environment variable atau menggunakan default "3000".
	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "3000"
	}

	// Mencatat bahwa server akan dimulai pada port yang ditentukan.
	zlog.Info().Msgf("Server is starting on port %s...", appPort)
	// Mulai mendengarkan request HTTP pada port yang ditentukan.
	// app.Listen bersifat blocking, akan berjalan terus sampai dihentikan atau error.
	startErr := app.Listen(fmt.Sprintf(":%s", appPort))
	if startErr != nil {
		// Jika terjadi error saat memulai server (misal: port sudah digunakan),
		// log error fatal dan hentikan aplikasi.
		zlog.Fatal().Err(startErr).Msg("Failed to start server")
	}
}
