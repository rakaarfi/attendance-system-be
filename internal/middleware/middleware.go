// internal/middleware/middleware.go
package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"  // Middleware untuk kompresi response (Gzip)
	"github.com/gofiber/fiber/v2/middleware/cors"      // Middleware untuk Cross-Origin Resource Sharing
	"github.com/gofiber/fiber/v2/middleware/limiter"   // Middleware untuk membatasi rate request
	"github.com/gofiber/fiber/v2/middleware/recover"   // Middleware untuk menangkap panic
	"github.com/gofiber/fiber/v2/middleware/requestid" // Middleware untuk menambahkan ID unik ke request
	"github.com/rs/zerolog"                            // Digunakan oleh logger request
	zlog "github.com/rs/zerolog/log"                   // Logger global Zerolog
)

// SetupGlobalMiddleware mendaftarkan middleware standar yang akan dijalankan
// untuk sebagian besar atau semua request ke aplikasi Fiber.
// Urutan pendaftaran middleware penting.
func SetupGlobalMiddleware(app *fiber.App) {
	// --- 1. Recover Middleware (Paling Awal) ---
	// Menangkap panic yang mungkin terjadi di handler atau middleware lain
	// agar server tidak crash. Mengembalikan response 500 Internal Server Error.
	// Harus didaftarkan sepagi mungkin.
	app.Use(recover.New())
	zlog.Info().Msg("Recover middleware registered")

	// --- 2. Request ID Middleware ---
	// Menambahkan header 'X-Request-ID' ke setiap request (jika belum ada)
	// dan menyimpannya di c.Locals("requestid"). Berguna untuk tracing log.
	app.Use(requestid.New())
	zlog.Info().Msg("RequestID middleware registered")

	// --- 3. CORS Middleware ---
	// Mengatur header Cross-Origin Resource Sharing. Penting agar frontend
	// yang berjalan di domain berbeda bisa berkomunikasi dengan API ini.
	app.Use(cors.New(cors.Config{
		// AllowOrigins: "*", // Izinkan semua origin (HATI-HATI di production!)
		// Ganti dengan daftar origin frontend Anda di production, pisahkan dengan koma.
		AllowOrigins: "http://localhost:5173, http://127.0.0.1:5173, https://frontend-domain-anda.com, http://localhost:3001",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",      // Metode HTTP yang diizinkan.
		AllowHeaders: "Origin, Content-Type, Accept, Authorization", // Header yang boleh dikirim oleh klien.
		// AllowCredentials: true, // Set true jika perlu mengirim cookie lintas domain.
	}))
	zlog.Info().Msg("CORS middleware registered")

	// --- 4. Rate Limiter Middleware ---
	// Membatasi jumlah request dari IP address yang sama dalam periode waktu tertentu.
	// Membantu mencegah serangan brute-force atau penyalahgunaan API.
	app.Use(limiter.New(limiter.Config{
		Max:        200,             // Maksimum 200 request...
		Expiration: 1 * time.Minute, // ...dalam periode 1 menit per IP.
		// KeyGenerator: func(c *fiber.Ctx) string { return c.Get("x-forwarded-for")}, // Gunakan jika di belakang reverse proxy/load balancer.
		LimiterMiddleware: limiter.SlidingWindow{}, // Algoritma rate limiting (Sliding Window).
	}))
	zlog.Info().Msg("Rate limiter middleware registered")

	// --- 5. Logger Request Middleware (Custom Zerolog) ---
	// Mencatat detail setiap request HTTP yang masuk setelah diproses middleware sebelumnya.
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now() // Waktu mulai request

		// Lanjutkan ke middleware/handler berikutnya
		err := c.Next() // Jika handler mengembalikan error, akan ditangkap di sini

		stop := time.Now()                      // Waktu selesai request
		latency := stop.Sub(start)              // Durasi pemrosesan request
		statusCode := c.Response().StatusCode() // Status HTTP response

		// Ambil Request ID yang ditambahkan oleh middleware requestid
		requestIDInterface := c.Locals("requestid")
		requestID := "" // Default string kosong
		if requestIDInterface != nil {
			// Lakukan type assertion yang aman
			if idStr, ok := requestIDInterface.(string); ok {
				requestID = idStr
			}
		}

		// Tentukan level log berdasarkan status code atau adanya error
		var logEvent *zerolog.Event
		if err != nil {
			// Jika ada error dari handler (akan ditangani juga oleh ErrorHandler global),
			// log sebagai warning/error di sini. ErrorHandler global akan memberikan response.
			logEvent = zlog.Warn().Err(err) // Atau Error() tergantung tingkat keparahan
		} else {
			// Log request sukses
			logEvent = zlog.Info() // Default Info
			if statusCode >= 500 {
				logEvent = zlog.Error() // Jika status 5xx, log sebagai Error
			} else if statusCode >= 400 {
				logEvent = zlog.Warn() // Jika status 4xx, log sebagai Warn
			}
		}

		// Bangun field-field log
		loggerWithFields := logEvent.
			Str("method", c.Method()).                      // Metode HTTP (GET, POST, etc.)
			Str("path", c.Path()).                          // Path request
			Int("status", statusCode).                      // Status code response
			Dur("latency", latency).                        // Durasi request
			Str("ip", c.IP()).                              // IP address klien
			Str("user_agent", c.Get(fiber.HeaderUserAgent)) // User agent klien

		// Tambahkan request ID jika ada
		if requestID != "" {
			loggerWithFields = loggerWithFields.Str("request_id", requestID)
		}

		// Tulis log
		loggerWithFields.Msg("Request handled")

		// Kembalikan error (jika ada) agar bisa ditangani oleh ErrorHandler global
		return err
	})
	zlog.Info().Msg("Request logger middleware registered")

	// --- 6. Compression Middleware ---
	// Mengompresi body response (Gzip) jika klien mendukungnya (header Accept-Encoding).
	// Menghemat bandwidth. Sebaiknya diletakkan mendekati akhir rantai.
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Kompresi cepat, ukuran sedikit lebih besar. Atau LevelDefault.
	}))
	zlog.Info().Msg("Compress middleware registered")

	// --- Middleware lain bisa ditambahkan di sini ---
	// Contoh:
	// app.Use(helmet.New()) // Middleware untuk menambahkan header keamanan (perlu library terpisah)
}
