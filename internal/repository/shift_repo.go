package repository

import (
	"context"
	"fmt"
	"time" // Digunakan untuk parsing/formatting jika perlu, meskipun DB type TIME

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn" // Untuk cek error code
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	zlog "github.com/rs/zerolog/log"
)

type shiftRepo struct {
	db *pgxpool.Pool
}

func NewShiftRepository(db *pgxpool.Pool) ShiftRepository {
	return &shiftRepo{db: db}
}

// CreateShift adds a new shift definition
func (r *shiftRepo) CreateShift(ctx context.Context, shift *models.Shift) (int, error) {
	query := `INSERT INTO shifts (name, start_time, end_time) VALUES ($1, $2, $3) RETURNING id`
	var shiftID int

	// Validasi format waktu sederhana (HH:MM:SS) - bisa lebih robust
	_, errStart := time.Parse("15:04:05", shift.StartTime)
	_, errEnd := time.Parse("15:04:05", shift.EndTime)
	if errStart != nil || errEnd != nil {
		zlog.Warn().Err(errStart).Err(errEnd).Msg("Invalid time format, use HH:MM:SS")
		return 0, fmt.Errorf("invalid time format, use HH:MM:SS")
	}

	err := r.db.QueryRow(ctx, query, shift.Name, shift.StartTime, shift.EndTime).Scan(&shiftID)
	if err != nil {
		zlog.Error().Err(err).Msg("Error creating shift")
		return 0, fmt.Errorf("error creating shift: %w", err)
	}
	zlog.Info().Int("shift_id", shiftID).Msg("Shift created successfully")
	return shiftID, nil
}

// GetShiftByID retrieves a shift by its ID
func (r *shiftRepo) GetShiftByID(ctx context.Context, id int) (*models.Shift, error) {
	query := `SELECT id, name, start_time, end_time, created_at, updated_at FROM shifts WHERE id = $1`
	shift := &models.Shift{}
	var startTime, endTime string // Baca sebagai string dari DB (tipe TIME)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&shift.ID,
		&shift.Name,
		&startTime,
		&endTime,
		&shift.CreatedAt,
		&shift.UpdatedAt,
	)
	if err != nil {
		// Handle pgx.ErrNoRows
		zlog.Warn().Err(err).Int("shift_id", id).Msg("Error getting shift by id")
		return nil, fmt.Errorf("error getting shift by id %d: %w", id, err)
	}
	// Assign string times ke struct
	shift.StartTime = startTime
	shift.EndTime = endTime

	zlog.Info().Int("shift_id", id).Msg("Shift retrieved successfully")
	return shift, nil
}

// GetAllShifts retrieves all shift definitions
func (r *shiftRepo) GetAllShifts(ctx context.Context) ([]models.Shift, error) {
	query := `SELECT id, name, start_time, end_time, created_at, updated_at FROM shifts ORDER BY name`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		zlog.Error().Err(err).Msg("Error getting all shifts")
		return nil, fmt.Errorf("error getting all shifts: %w", err)
	}
	defer rows.Close()

	shifts := []models.Shift{}
	for rows.Next() {
		var shift models.Shift
		var startTime, endTime string
		if err := rows.Scan(
			&shift.ID,
			&shift.Name,
			&startTime,
			&endTime,
			&shift.CreatedAt,
			&shift.UpdatedAt); err != nil {
			zlog.Warn().Err(err).Msg("Error scanning shift row") // Log error but continue processing other rows
			continue
		}
		shift.StartTime = startTime
		shift.EndTime = endTime
		shifts = append(shifts, shift)
	}

	if err = rows.Err(); err != nil {
		zlog.Error().Err(err).Msg("Error iterating shift rows")
		return nil, fmt.Errorf("error iterating shift rows: %w", err)
	}

	zlog.Info().Int("record_count", len(shifts)).Msg("Shifts retrieved successfully")
	return shifts, nil
}

// UpdateShift modifies an existing shift
func (r *shiftRepo) UpdateShift(ctx context.Context, shift *models.Shift) error {
	query := `UPDATE shifts SET name = $1, start_time = $2, end_time = $3, updated_at = CURRENT_TIMESTAMP
              WHERE id = $4`

	// Validasi format waktu
	_, errStart := time.Parse("15:04:05", shift.StartTime)
	_, errEnd := time.Parse("15:04:05", shift.EndTime)
	if errStart != nil || errEnd != nil {
		zlog.Warn().Err(errStart).Err(errEnd).Msg("Invalid time format, use HH:MM:SS")
		return fmt.Errorf("invalid time format, use HH:MM:SS")
	}

	tag, err := r.db.Exec(ctx, query, shift.Name, shift.StartTime, shift.EndTime, shift.ID)
	if err != nil {
		zlog.Error().Err(err).Int("shift_id", shift.ID).Msg("Error updating shift")
		return fmt.Errorf("error updating shift id %d: %w", shift.ID, err)
	}
	if tag.RowsAffected() == 0 {
		zlog.Info().Int("shift_id", shift.ID).Msg("No rows updated")
		return pgx.ErrNoRows // Kembalikan error standar jika tidak ada row yang terupdate
	}
	zlog.Info().Int("shift_id", shift.ID).Msg("Shift updated successfully")
	return nil
}

// DeleteShift removes a shift definition
func (r *shiftRepo) DeleteShift(ctx context.Context, id int) error {
	query := `DELETE FROM shifts WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		// Cek foreign key constraint violation (code 23503)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23503" {
			zlog.Warn().Err(err).Int("shift_id", id).Msg("Cannot delete shift: it is still referenced by user schedules")
			return fmt.Errorf("cannot delete shift: it is still referenced by user schedules")
		}
		zlog.Error().Err(err).Int("shift_id", id).Msg("Error deleting shift")
		return fmt.Errorf("error deleting shift id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		zlog.Info().Int("shift_id", id).Msg("No shift deleted")
		return pgx.ErrNoRows // Kembalikan error standar jika tidak ada row yang terhapus
	}
	zlog.Info().Int("shift_id", id).Msg("Shift deleted successfully")
	return nil
}
