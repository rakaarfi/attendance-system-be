// internal/logger/logger.go
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath" // Untuk manipulasi path file dan direktori
	"strconv"       // Untuk konversi string (dari env vars) ke bool/int
	"time"

	"github.com/rs/zerolog"            // Core library Zerolog
	"github.com/rs/zerolog/log"        // Akses ke logger global Zerolog
	"gopkg.in/natefinch/lumberjack.v2" // Library untuk rotasi file log
)

// SetupLogger mengkonfigurasi logger global Zerolog berdasarkan environment variables.
// Mendukung output ke konsol (human-readable atau JSON) dan secara opsional ke file log
// dengan rotasi otomatis menggunakan lumberjack.
//
// Mengembalikan io.Closer yang merepresentasikan file logger (jika aktif),
// yang harus ditutup menggunakan 'defer' di fungsi main untuk memastikan buffer ditulis.
// Mengembalikan nil jika file logging tidak aktif atau gagal diinisialisasi.
//
// Variabel Environment yang didukung:
//   - LOG_LEVEL: Tingkat log minimum (trace, debug, info, warn, error, fatal, panic). Default: info.
//   - LOG_FORMAT: Format output konsol ('json' atau lainnya untuk human-readable). Default: human-readable.
//   - LOG_FILE_ENABLED: Aktifkan logging ke file ('true' atau 'false'). Default: false.
//   - LOG_FILE_PATH: Path lengkap ke file log. Default: ./logs/app.log.
//   - LOG_FILE_MAX_SIZE_MB: Ukuran maksimum file log (MB) sebelum dirotasi. Default: 100.
//   - LOG_FILE_MAX_BACKUPS: Jumlah maksimum file log lama yang disimpan. Default: 5.
//   - LOG_FILE_MAX_AGE_DAYS: Usia maksimum file log lama (hari) sebelum dihapus. Default: 30.
//   - LOG_FILE_COMPRESS: Kompres file log lama ('true' atau 'false'). Default: false.
func SetupLogger() io.Closer {
	// --- Konfigurasi Tingkat Log Global ---
	logLevelStr := os.Getenv("LOG_LEVEL")
	logLevel, err := zerolog.ParseLevel(logLevelStr)
	// Jika LOG_LEVEL tidak valid atau kosong, gunakan 'info' sebagai default.
	if err != nil || logLevelStr == "" {
		logLevel = zerolog.InfoLevel
		// Gunakan fmt ke Stderr karena logger mungkin belum siap
		fmt.Fprintf(os.Stderr, "[WARN] Invalid or missing LOG_LEVEL env var ('%s'), using default: %s\n", logLevelStr, logLevel.String())
	}
	zerolog.SetGlobalLevel(logLevel) // Terapkan level log secara global

	// --- Konfigurasi Writer (Output Tujuan Log) ---
	var writers []io.Writer // Slice untuk menampung semua tujuan output (konsol, file)

	// Konfigurasi Console Writer
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat != "json" {
		// Format human-readable, cocok untuk development di terminal.
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
		writers = append(writers, consoleWriter)
	} else {
		// Format JSON standar, cocok untuk dikonsumsi oleh sistem log management.
		writers = append(writers, os.Stderr) // Output JSON langsung ke Stderr
	}

	// --- Konfigurasi File Writer (Opsional, dengan Rotasi) ---
	var fileCloser io.Closer                                              // Untuk menyimpan handle lumberjack agar bisa di-Close()
	logFileEnabled, _ := strconv.ParseBool(os.Getenv("LOG_FILE_ENABLED")) // Default false jika error/kosong

	if logFileEnabled {
		// Tentukan path file log, gunakan default jika tidak di-set.
		logFilePath := os.Getenv("LOG_FILE_PATH")
		if logFilePath == "" {
			logFilePath = "./logs/app.log"
			// Tidak perlu log warning di sini karena logger utama belum tentu siap,
			// pesan file logging enabled di bawah sudah cukup.
		}

		// Pastikan direktori untuk file log ada.
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0744); err != nil { // 0744: rwxr--r--
			// Jika gagal membuat direktori, log error ke Stderr dan lewati file logging.
			fmt.Fprintf(os.Stderr, "[ERROR] Can't create log directory '%s': %v. File logging disabled.\n", logDir, err)
		} else {
			// Baca konfigurasi rotasi dari env vars atau gunakan default.
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

			// Inisialisasi Lumberjack untuk menangani rotasi file.
			fileWriter := &lumberjack.Logger{
				Filename:   logFilePath,  // Nama file log utama.
				MaxSize:    maxSizeMB,    // Maksimum ukuran (MB) sebelum file dirotasi.
				MaxBackups: maxBackups,   // Maksimum jumlah file lama (.log.gz jika kompres) yang disimpan.
				MaxAge:     maxAgeDays,   // Maksimum usia (hari) file lama sebelum dihapus.
				Compress:   compressLogs, // Apakah file lama dikompres (gzip).
			}
			writers = append(writers, fileWriter) // Tambahkan file writer ke daftar output.
			fileCloser = fileWriter               // Simpan handle untuk di-Close nanti.
		}
	}

	// --- Gabungkan Semua Writer Menjadi Satu ---
	// MultiLevelWriter memungkinkan log ditulis ke semua writer yang ada di slice 'writers'.
	multiWriter := zerolog.MultiLevelWriter(writers...)

	// --- Atur Logger Global Zerolog ---
	// Buat instance logger baru yang menulis ke multiWriter.
	// .With() memulai context builder untuk field global.
	// .Timestamp() menambahkan field timestamp ke semua log.
	// .Caller() menambahkan field caller (nama file:baris) ke semua log.
	log.Logger = zerolog.New(multiWriter).With().Timestamp().Caller().Logger()

	// Log pesan konfirmasi setelah logger utama siap.
	log.Info().Msgf("Global logger initialized. Level: %s. Format: %s. File Logging: %t.",
		zerolog.GlobalLevel().String(),
		logFormat,                           // Atau tentukan format console secara eksplisit
		logFileEnabled && fileCloser != nil) // Konfirmasi file logging benar-benar aktif

	// Kembalikan fileCloser (bisa nil) agar bisa ditutup di main.
	return fileCloser
}
