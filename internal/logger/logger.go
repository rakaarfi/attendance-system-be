// internal/logger/logger.go
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogger configures the global zerolog logger.
// It supports output to the console (either JSON or human-readable format)
// and optionally to a log file with rotation via lumberjack.
//
// Returns:
//   - io.Closer: A closer for the file logger if enabled, which should be closed in main using `defer`.
//   - nil: If file logging is disabled or fails.
//
// Environment Variables:
//   - LOG_LEVEL             (e.g., "debug", "info", "warn", "error")
//   - LOG_FORMAT            ("json" or any other value for human-readable)
//   - LOG_FILE_ENABLED      ("true" or "false")
//   - LOG_FILE_PATH         (e.g., "./logs/app.log")
//   - LOG_FILE_MAX_SIZE_MB  (e.g., "100")
//   - LOG_FILE_MAX_BACKUPS  (e.g., "5")
//   - LOG_FILE_MAX_AGE_DAYS (e.g., "30")
//   - LOG_FILE_COMPRESS     ("true" or "false")
func SetupLogger() io.Closer {
	// --- Level Configuration  ---
	logLevelStr := os.Getenv("LOG_LEVEL")
	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil || logLevelStr == "" {
		logLevel = zerolog.InfoLevel // Default to INFO if not set or invalid
		fmt.Fprintf(os.Stderr, "[WARN] Invalid or missing LOG_LEVEL env var, using default: %s\n", logLevel.String())
	}
	zerolog.SetGlobalLevel(logLevel)

	// --- Writer Configuration ---
	var writers []io.Writer // Will hold all writers (console, file, etc.)

	// --- Console Writer Configuration ---
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat != "json" {
		// Use human-readable format for console
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		}
		writers = append(writers, consoleWriter)
		log.Info().Msg("Using console log output (human readable)")
	} else {
		// Use JSON format
		writers = append(writers, os.Stderr)
		log.Info().Msg("Using console log output (JSON)")
	}

	// --- File Writer with Log Rotation ---
	// Log rotation is used for:
	// 1. set max sized of log files
	// 2. set max number of log files
	// 3. set max number of days to keep log files
	// 4. set whether to compress log files
	var fileCloser io.Closer
	logFileEnabledStr := os.Getenv("LOG_FILE_ENABLED")
	logFileEnabled, _ := strconv.ParseBool(logFileEnabledStr)

	if logFileEnabled {
		logFilePath := os.Getenv("LOG_FILE_PATH")
		if logFilePath == "" {
			logFilePath = "./logs/app.log" // Default path if not set
			log.Warn().Msgf("LOG_FILE_PATH not set, using default: %s", logFilePath)
		}

		// Ensure log directory exists
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0744); err != nil {
			log.Error().Err(err).Str("path", logDir).Msg("Can't create log directory")
			// Skip file logging if the directory cannot be created
		} else {
			// Load file rotation settings from environment or use defaults
			maxSizeMB, _ := strconv.Atoi(os.Getenv("LOG_FILE_MAX_SIZE_MB"))
			if maxSizeMB <= 0 {
				maxSizeMB = 100
			}
			maxBackups, _ := strconv.Atoi(os.Getenv("LOG_FILE_MAX_BACKUPS"))
			if maxBackups <= 0 {
				maxBackups = 5
			}
			maxAgeDays, _ := strconv.Atoi(os.Getenv("LOG_FILE_MAX_AGE_DAYS"))
			if maxAgeDays <= 0 {
				maxAgeDays = 30
			}
			compressLogs, _ := strconv.ParseBool(os.Getenv("LOG_FILE_COMPRESS"))

			// Create lumberjack logger with rotation
			fileWriter := &lumberjack.Logger{
				Filename:   logFilePath,
				MaxSize:    maxSizeMB,  // megabytes
				MaxBackups: maxBackups, // number of old files to retain
				MaxAge:     maxAgeDays, // days
				Compress:   compressLogs,
			}
			writers = append(writers, fileWriter)
			fileCloser = fileWriter // For cleanup later

			log.Info().
				Str("path", logFilePath).
				Int("max_size_mb", maxSizeMB).
				Int("max_backups", maxBackups).
				Int("max_age_days", maxAgeDays).
				Bool("compress", compressLogs).
				Msg("File logging enabled with rotation")
		}
	} else {
		log.Info().Msg("File logging disabled (set LOG_FILE_ENABLED=true to enable)")
	}

	// --- Combine All Writers ---
	multiWriter := zerolog.MultiLevelWriter(writers...)

	// --- Set Global Logger ---
	// Includes timestamp and caller (file:line) in every log entry
	log.Logger = zerolog.New(multiWriter).
		With().
		Timestamp().
		Caller().
		Logger()

	log.Info().Msgf("Global logger initialized with level: %s", zerolog.GlobalLevel().String())

	// --- Return fileCloser for cleanup in main ---
	return fileCloser
}
