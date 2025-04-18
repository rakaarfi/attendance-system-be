package configs

import (
	"os"

	"github.com/joho/godotenv"
	zlog "github.com/rs/zerolog/log"
)

func LoadConfig() {
	// Cari file .env di direktori saat ini atau parent
	err := godotenv.Load()
	if err != nil {
		// Tidak masalah jika .env tidak ada, mungkin variabel di-set langsung di environment
		zlog.Warn().Msg("No .env file found, reading environment variables directly.")
	}

	// Anda bisa menambahkan validasi di sini untuk memastikan variabel penting ada
	requiredVars := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "APP_PORT", "JWT_SECRET"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			zlog.Fatal().Str("var", v).Msg("Environment variable is not set.")
		}
	}
	zlog.Info().Msg("All required environment variables are set.")
}
