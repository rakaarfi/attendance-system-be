// internal/utils/pagination.go
package utils

import (
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
	zlog "github.com/rs/zerolog/log"
)

const DefaultPage = 1
const DefaultLimit = 10
const MaxLimit = 100 // Batas maksimum untuk limit

// PaginationQuery menampung parameter pagination yang sudah divalidasi
type PaginationQuery struct {
	Page   int
	Limit  int
	Offset int
}

// ParsePaginationParams membaca dan memvalidasi parameter page & limit dari query string
func ParsePaginationParams(c *fiber.Ctx) PaginationQuery {
	page, err := strconv.Atoi(c.Query("page", strconv.Itoa(DefaultPage)))
	if err != nil || page < 1 {
		zlog.Warn().Str("page_query", c.Query("page")).Msg("Invalid page query parameter, using default")
		page = DefaultPage
	}

	limit, err := strconv.Atoi(c.Query("limit", strconv.Itoa(DefaultLimit)))
	if err != nil || limit < 1 {
		zlog.Warn().Str("limit_query", c.Query("limit")).Msg("Invalid limit query parameter, using default")
		limit = DefaultLimit
	}

	if limit > MaxLimit {
		zlog.Warn().Int("requested_limit", limit).Int("max_limit", MaxLimit).Msg("Requested limit exceeds maximum, capping")
		limit = MaxLimit
	}

	offset := (page - 1) * limit

	return PaginationQuery{
		Page:   page,
		Limit:  limit,
		Offset: offset,
	}
}

// PaginationMeta berisi metadata untuk response pagination
type PaginationMeta struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	TotalItems  int `json:"total_items"`
	TotalPages  int `json:"total_pages"`
}

// BuildPaginationMeta membuat metadata pagination
func BuildPaginationMeta(totalItems, limit, page int) PaginationMeta {
	totalPages := 0
	if totalItems > 0 && limit > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(limit)))
	}
	return PaginationMeta{
		CurrentPage: page,
		PerPage:     limit,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
	}
}

// PaginatedResponse adalah struktur generik untuk response pagination
// Membutuhkan Go 1.18+
type PaginatedResponse[T any] struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    []T            `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}

// NewPaginatedResponse membuat instance PaginatedResponse
func NewPaginatedResponse[T any](message string, data []T, meta PaginationMeta) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	}
}
