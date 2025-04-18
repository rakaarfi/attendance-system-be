// internal/middleware/middleware.go
package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

func SetupGlobalMiddleware(app *fiber.App) {
	// 1. Recover (Paling awal)
	app.Use(recover.New())
	zlog.Info().Msg("Recover middleware registered")

	// 2. Request ID
	app.Use(requestid.New())
	zlog.Info().Msg("RequestID middleware registered")

	// 3. CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Ganti di production!
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))
	zlog.Info().Msg("CORS middleware registered")

	// 4. Rate Limiter
	app.Use(limiter.New(limiter.Config{
		Max:        200,             // Maks 200 request...
		Expiration: 1 * time.Minute, // ...per menit per IP
		// KeyGenerator: func(c *fiber.Ctx) string { return c.Get("x-forwarded-for")}, // Sesuaikan jika di belakang proxy
		LimiterMiddleware: limiter.SlidingWindow{},
	}))
	zlog.Info().Msg("Rate limiter middleware registered")

	// 5. Logger Request
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		stop := time.Now()
		latency := stop.Sub(start)
		statusCode := c.Response().StatusCode()
		requestIDInterface := c.Locals("requestid")
		requestID := ""
		if requestIDInterface != nil {
			if idStr, ok := requestIDInterface.(string); ok {
				requestID = idStr
			}
		}

		var logEvent *zerolog.Event
		if err != nil {
			logEvent = zlog.Warn().Err(err)
		} else {
			logEvent = zlog.Info()
			if statusCode >= 500 {
				logEvent = zlog.Error()
			} else if statusCode >= 400 {
				logEvent = zlog.Warn()
			}
		}

		loggerWithFields := logEvent.
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", statusCode).
			Dur("latency", latency).
			Str("ip", c.IP()).
			Str("user_agent", c.Get(fiber.HeaderUserAgent))

		if requestID != "" {
			loggerWithFields = loggerWithFields.Str("request_id", requestID)
		}

		loggerWithFields.Msg("Request handled")

		return err
	})
	zlog.Info().Msg("Request logger middleware registered")

	// 6. Compression
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Atau LevelDefault
	}))
	zlog.Info().Msg("Compress middleware registered")

	// Middleware lain bisa ditambahkan di sini
}
