// internal/database/postgres.go
package database

import (
	"context" // Paket standar untuk mengelola context, terutama untuk timeout dan cancellation.
	"fmt"     // Paket standar untuk formatting string.
	"os"      // Paket standar untuk interaksi OS, digunakan di sini untuk membaca environment variables.
	"time"    // Paket standar untuk fungsionalitas waktu (durasi, timeout).

	"github.com/jackc/pgx/v5/pgxpool" // Driver PostgreSQL modern dan efisien, fokus pada connection pool.
	zlog "github.com/rs/zerolog/log"  // Logger global Zerolog.
)

// NewPgxPool 
// - membuat dan mengembalikan instance baru dari connection pool pgxpool (*pgxpool.Pool).
// - membaca konfigurasi database dari environment variables.
// - melakukan ping ke database untuk memastikan koneksi awal berhasil.
func NewPgxPool() (*pgxpool.Pool, error) {
	// --- Langkah 1: Baca Konfigurasi Database dari Environment Variables ---
	// Mengambil detail koneksi dari environment. Pastikan variabel ini sudah di-set
	// (misalnya melalui .env dan configs.LoadConfig() sebelum memanggil fungsi ini).
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbSSLMode := os.Getenv("DB_SSLMODE") // 'disable', 'require', 'verify-full', dll.

	// Membuat Data Source Name (DSN) string sesuai format yang dibutuhkan pgx.
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	// --- Langkah 2: Parse Konfigurasi DSN ---
	// Mengubah DSN string menjadi struct konfigurasi *pgxpool.Config.
	// Ini juga melakukan validasi dasar pada format DSN.
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		// Jika DSN tidak valid, kembalikan error sebelum mencoba membuat pool.
		zlog.Error().Err(err).Str("dsn_prefix", fmt.Sprintf("host=%s port=%s user=%s dbname=%s ...", dbHost, dbPort, dbUser, dbName)).Msg("Unable to parse database DSN")
		return nil, fmt.Errorf("unable to parse database config: %w", err) // %w membungkus error asli
	}

	// --- Langkah 3: Konfigurasi Detail Connection Pool (Opsional tapi Direkomendasikan) ---
	// Menyesuaikan parameter pool untuk mengontrol jumlah koneksi, masa hidup, dll.
	// Nilai default mungkin cukup untuk development, tapi perlu disesuaikan untuk production.
	config.MaxConns = 10                               // Jumlah maksimum koneksi (aktif + idle) yang diizinkan dalam pool.
	config.MinConns = 2                                // Jumlah minimum koneksi yang coba dipertahankan idle oleh pool.
	config.MaxConnLifetime = time.Hour                 // Durasi maksimum sebuah koneksi bisa digunakan sebelum ditutup paksa (membantu load balancing, mencegah koneksi basi).
	config.MaxConnIdleTime = 30 * time.Minute          // Durasi maksimum koneksi idle bisa bertahan sebelum ditutup.
	config.HealthCheckPeriod = time.Minute             // Seberapa sering pool memeriksa koneksi idle yang 'rusak'.
	config.ConnConfig.ConnectTimeout = 5 * time.Second // Waktu maksimum untuk mencoba membuat koneksi *baru*.

	// --- Langkah 4: Buat Connection Pool ---
	// Mencoba membuat pool koneksi menggunakan konfigurasi yang sudah di-parse dan disesuaikan.
	// context.Background() digunakan karena pembuatan pool ini terjadi di luar konteks request HTTP.
	zlog.Info().Msg("Attempting to create database connection pool...")
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		// Jika pembuatan pool gagal (misal: masalah jaringan awal, autentikasi salah), kembalikan error.
		zlog.Error().Err(err).Msg("Unable to create database connection pool")
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// --- Langkah 5: Verifikasi Koneksi Awal (Ping) ---
	// Melakukan 'ping' ke database menggunakan salah satu koneksi dari pool.
	// Ini memastikan bahwa setidaknya satu koneksi dapat berhasil dibuat dan berkomunikasi dengan DB.
	// Menggunakan context dengan timeout agar proses ping tidak menggantung selamanya.
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Timeout 5 detik untuk ping
	defer cancel()                                                              // Pastikan context dibatalkan setelah selesai untuk melepaskan resource

	zlog.Info().Msg("Pinging database to verify connection...")
	if err := pool.Ping(pingCtx); err != nil {
		// Jika ping gagal (DB tidak running, firewall, kredensial salah, dll.):
		pool.Close() // Penting: Tutup pool yang sudah terlanjur dibuat tapi tidak bisa digunakan.
		zlog.Error().Err(err).Msg("Unable to ping database")
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	// --- Langkah 6: Koneksi Berhasil ---
	// Jika semua langkah berhasil, catat pesan sukses dan kembalikan pool yang siap pakai.
	zlog.Info().Msg("Successfully connected to PostgreSQL database and verified with ping!")
	return pool, nil // Kembalikan pool dan error nil
}
