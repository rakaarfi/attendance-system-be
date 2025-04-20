// internal/utils/pagination.go
package utils

import (
	"math"    // Digunakan untuk math.Ceil (pembulatan ke atas)
	"strconv" // Untuk konversi string ke integer

	"github.com/gofiber/fiber/v2"    // Framework Fiber untuk context (c *fiber.Ctx)
	zlog "github.com/rs/zerolog/log" // Logger global Zerolog
)

// Konstanta untuk nilai default dan batasan pagination.
const (
	DefaultPage  = 1   // Halaman default jika tidak dispesifikasikan.
	DefaultLimit = 10  // Jumlah item default per halaman.
	MaxLimit     = 100 // Batas maksimum item per halaman untuk mencegah request berlebihan.
)

// PaginationQuery menampung parameter pagination yang sudah dibersihkan dan divalidasi.
type PaginationQuery struct {
	Page   int // Nomor halaman saat ini (dimulai dari 1).
	Limit  int // Jumlah item per halaman.
	Offset int // Jumlah item yang dilewati (untuk query database: (Page - 1) * Limit).
}

// ParsePaginationParams membaca parameter 'page' dan 'limit' dari query string request Fiber.
// Memberikan nilai default jika parameter tidak ada atau tidak valid.
// Memberlakukan batas MaxLimit.
// Menghitung offset yang sesuai.
// Mengembalikan struct PaginationQuery yang siap digunakan.
func ParsePaginationParams(c *fiber.Ctx) PaginationQuery {
	// Ambil 'page', gunakan DefaultPage jika kosong/error.
	pageStr := c.Query("page", strconv.Itoa(DefaultPage))
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 { // Halaman tidak boleh kurang dari 1.
		zlog.Warn().Str("page_query", pageStr).Msg("Invalid page query parameter, using default")
		page = DefaultPage
	}

	// Ambil 'limit', gunakan DefaultLimit jika kosong/error.
	limitStr := c.Query("limit", strconv.Itoa(DefaultLimit))
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 { // Limit tidak boleh kurang dari 1.
		zlog.Warn().Str("limit_query", limitStr).Msg("Invalid limit query parameter, using default")
		limit = DefaultLimit
	}

	// Batasi limit ke MaxLimit.
	if limit > MaxLimit {
		zlog.Warn().Int("requested_limit", limit).Int("max_limit", MaxLimit).Msg("Requested limit exceeds maximum, capping")
		limit = MaxLimit
	}

	// Hitung offset untuk query database.
	offset := (page - 1) * limit

	return PaginationQuery{
		Page:   page,
		Limit:  limit,
		Offset: offset,
	}
}

// PaginationMeta berisi metadata yang dikirim dalam response pagination.
// Berguna untuk frontend membangun navigasi halaman.
type PaginationMeta struct {
	CurrentPage int `json:"current_page"` // Halaman saat ini.
	PerPage     int `json:"per_page"`     // Jumlah item per halaman yang digunakan (limit).
	TotalItems  int `json:"total_items"`  // Total jumlah item di semua halaman.
	TotalPages  int `json:"total_pages"`  // Total jumlah halaman yang tersedia.
}

// BuildPaginationMeta menghitung dan membuat struct PaginationMeta.
// Menerima total item, limit yang digunakan, dan halaman saat ini.
func BuildPaginationMeta(totalItems, limit, page int) PaginationMeta {
	totalPages := 0
	// Hitung total halaman, hindari pembagian dengan nol.
	if totalItems > 0 && limit > 0 {
		// Gunakan math.Ceil untuk pembulatan ke atas (misal: 11 item / 10 limit = 2 halaman).
		totalPages = int(math.Ceil(float64(totalItems) / float64(limit)))
	}
	return PaginationMeta{
		CurrentPage: page,
		PerPage:     limit,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
	}
}

// PaginatedResponse adalah struktur generik (menggunakan Go Generics 1.18+)
// untuk membungkus data hasil pagination dan metadatanya dalam response JSON standar.
// 'T' adalah tipe data dari item dalam slice 'Data' (misal: models.User, models.Shift).
type PaginatedResponse[T any] struct {
	Success bool           `json:"success"` // Status keberhasilan request.
	Message string         `json:"message"` // Pesan deskriptif.
	Data    []T            `json:"data"`    // Slice data untuk halaman saat ini.
	Meta    PaginationMeta `json:"meta"`    // Metadata pagination.
}

// NewPaginatedResponse adalah fungsi helper untuk membuat instance PaginatedResponse.
// Lebih ringkas daripada membuat struct literal secara langsung di handler.
func NewPaginatedResponse[T any](message string, data []T, meta PaginationMeta) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	}
}
