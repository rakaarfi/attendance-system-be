package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn" // Untuk cek error code
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	zlog "github.com/rs/zerolog/log"
)

type scheduleRepo struct {
	db *pgxpool.Pool
}

func NewScheduleRepository(db *pgxpool.Pool) ScheduleRepository {
	return &scheduleRepo{db: db}
}

const dateLayout = "2006-01-02" // YYYY-MM-DD

// CreateSchedule assigns a shift to a user on a specific date
func (r *scheduleRepo) CreateSchedule(ctx context.Context, schedule *models.UserSchedule) (int, error) {
	zlog.Info().Int("user_id", schedule.UserID).Int("shift_id", schedule.ShiftID).Str("date", schedule.Date).Msg("Creating schedule for user and date")

	query := `INSERT INTO user_schedules (user_id, shift_id, date) VALUES ($1, $2, $3) RETURNING id`
	var scheduleID int

	// Parse tanggal dari string ke time.Time untuk validasi dan insert
	scheduleDate, err := time.Parse(dateLayout, schedule.Date)
	if err != nil {
		zlog.Warn().Err(err).Str("date", schedule.Date).Msg("Invalid date format for schedule, use YYYY-MM-DD")
		return 0, fmt.Errorf("invalid date format for schedule, use YYYY-MM-DD: %w", err)
	}

	err = r.db.QueryRow(ctx, query, schedule.UserID, schedule.ShiftID, scheduleDate).Scan(&scheduleID)
	if err != nil {
		// Cek unique constraint violation (user_id, date)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			zlog.Warn().Err(err).Int("user_id", schedule.UserID).Str("date", schedule.Date).Msg("User already has a schedule on this date")
			return 0, fmt.Errorf("user %d already has a schedule on %s", schedule.UserID, schedule.Date)
		}
		// Cek foreign key constraint violation (misal user_id atau shift_id tidak ada)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23503" {
			zlog.Warn().Err(err).Int("user_id", schedule.UserID).Int("shift_id", schedule.ShiftID).Msg("Invalid user_id or shift_id")
			return 0, fmt.Errorf("invalid user_id (%d) or shift_id (%d)", schedule.UserID, schedule.ShiftID)
		}
		zlog.Error().Err(err).Int("user_id", schedule.UserID).Int("shift_id", schedule.ShiftID).Str("date", schedule.Date).Msg("Error creating schedule")
		return 0, fmt.Errorf("error creating schedule: %w", err)
	}
	zlog.Info().Int("schedule_id", scheduleID).Int("user_id", schedule.UserID).Int("shift_id", schedule.ShiftID).Str("date", schedule.Date).Msg("Schedule created successfully")
	return scheduleID, nil
}

// GetScheduleByUserAndDate retrieves a specific schedule
func (r *scheduleRepo) GetScheduleByUserAndDate(ctx context.Context, userID int, date time.Time) (*models.UserSchedule, error) {
	zlog.Info().Int("user_id", userID).Str("date", date.Format(dateLayout)).Msg("Retrieving schedule for user and date")

	query := `
        SELECT us.id, us.user_id, us.shift_id, us.date, us.created_at,
               s.id as shiftid, s.name as shiftname, s.start_time, s.end_time
        FROM user_schedules us
        JOIN shifts s ON us.shift_id = s.id
        WHERE us.user_id = $1 AND us.date = $2`

	schedule := &models.UserSchedule{Shift: &models.Shift{}}
	var scheduleDate time.Time
	var startTime, endTime string

	err := r.db.QueryRow(ctx, query, userID, date).Scan(
		&schedule.ID,
		&schedule.UserID,
		&schedule.ShiftID,
		&scheduleDate,
		&schedule.CreatedAt,
		&schedule.Shift.ID,
		&schedule.Shift.Name,
		&startTime,
		&endTime,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			zlog.Warn().Int("user_id", userID).Str("date", date.Format(dateLayout)).Msg("No schedule found for user and date")
			return nil, nil
		}
		zlog.Error().Err(err).Int("user_id", userID).Str("date", date.Format(dateLayout)).Msg("Error getting schedule")
		return nil, fmt.Errorf("error getting schedule for user %d on %s: %w", userID, date.Format(dateLayout), err)
	}

	schedule.Date = scheduleDate.Format(dateLayout)
	schedule.Shift.StartTime = startTime
	schedule.Shift.EndTime = endTime

	zlog.Info().Int("user_id", userID).Str("date", scheduleDate.Format(dateLayout)).Msg("Schedule retrieved successfully")
	return schedule, nil
}

// GetSchedulesByUser retrieves schedules for a user within a date range
func (r *scheduleRepo) GetSchedulesByUser(ctx context.Context, userID int, startDate, endDate time.Time, page, limit int) (schedules []models.UserSchedule, totalCount int, err error) {
	// 1. Count Total
	countQuery := `SELECT COUNT(*) FROM user_schedules WHERE user_id = $1 AND date >= $2 AND date <= $3`
	err = r.db.QueryRow(ctx, countQuery, userID, startDate, endDate).Scan(&totalCount)
	if err != nil {
		err = fmt.Errorf("error counting schedules for user %d: %w", userID, err)
		return
	}
	if totalCount == 0 {
		schedules = []models.UserSchedule{}
		return
	}

	// 2. Calculate Offset
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// 3. Query Data with JOIN, Filters, ORDER BY, LIMIT, OFFSET
	query := `
        SELECT us.id, us.user_id, us.shift_id, us.date, us.created_at,
               s.id as shiftid, s.name as shiftname, s.start_time, s.end_time
        FROM user_schedules us
        JOIN shifts s ON us.shift_id = s.id
        WHERE us.user_id = $1 AND us.date >= $2 AND us.date <= $3
        ORDER BY us.date ASC -- ORDER BY penting
        LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, query, userID, startDate, endDate, limit, offset)
	if err != nil {
		err = fmt.Errorf("error getting paginated schedules for user %d: %w", userID, err)
		return
	}
	defer rows.Close()

	// 4. Scan Results
	schedules = []models.UserSchedule{}
	for rows.Next() {
		var schedule models.UserSchedule
		schedule.Shift = &models.Shift{} // Init nested struct
		var scheduleDate time.Time
		var startTime, endTime string

		scanErr := rows.Scan(
			&schedule.ID,
			&schedule.UserID,
			&schedule.ShiftID,
			&scheduleDate,
			&schedule.CreatedAt,
			&schedule.Shift.ID,
			&schedule.Shift.Name,
			&startTime,
			&endTime,
		)
		if scanErr != nil {
			zlog.Warn().Err(scanErr).Int("user_id", userID).Msg("Error scanning user schedule row (paginated)")
			// Mungkin return error di sini
			err = fmt.Errorf("error scanning schedule row: %w", scanErr)
			return
		}
		schedule.Date = scheduleDate.Format(dateLayout)
		schedule.Shift.StartTime = startTime
		schedule.Shift.EndTime = endTime
		schedules = append(schedules, schedule)
	}

	if err = rows.Err(); err != nil {
		err = fmt.Errorf("error iterating schedule rows: %w", err)
		return
	}
	return // schedules, totalCount, nil error implicitly returned
}

// Tambahkan fungsi lain jika perlu (misal: GetSchedulesByDateRangeForAllUsers, UpdateSchedule, DeleteSchedule)

func (r *scheduleRepo) GetSchedulesByDateRangeForAllUsers(ctx context.Context, startDate, endDate time.Time, page, limit int) (schedules []models.UserSchedule, totalCount int, err error) {
	// 1. Count Total
	countQuery := `SELECT COUNT(*) FROM user_schedules WHERE date >= $1 AND date <= $2`
	err = r.db.QueryRow(ctx, countQuery, startDate, endDate).Scan(&totalCount)
	if err != nil {
		err = fmt.Errorf("error counting all schedules: %w", err)
		return
	}
	if totalCount == 0 {
		schedules = []models.UserSchedule{}
		return
	}

	// 2. Calculate Offset
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// 3. Query Data
	query := `
		SELECT us.id, us.user_id, us.shift_id, us.date, us.created_at,
		       s.id as shiftid, s.name as shiftname, s.start_time, s.end_time,
               u.id as userid, u.username, u.email, u.first_name, u.last_name -- Tambahkan info user jika perlu di response ini
		FROM user_schedules us
		JOIN shifts s ON us.shift_id = s.id
        JOIN users u ON us.user_id = u.id -- JOIN users
		WHERE us.date >= $1 AND us.date <= $2
		ORDER BY us.date ASC, u.username ASC -- ORDER BY penting
        LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, query, startDate, endDate, limit, offset)
	if err != nil {
		err = fmt.Errorf("error getting paginated all schedules: %w", err)
		return
	}
	defer rows.Close()

	// 4. Scan Results (termasuk field user jika ditambahkan di query)
	schedules = []models.UserSchedule{}
	for rows.Next() {
		var schedule models.UserSchedule
		schedule.Shift = &models.Shift{} // Init nested struct
		schedule.User = &models.User{}
		var scheduleDate time.Time
		var startTime, endTime string
		scanErr := rows.Scan(
			&schedule.ID, 
			&schedule.UserID, 
			&schedule.ShiftID, 
			&scheduleDate, 
			&schedule.CreatedAt,
			&schedule.Shift.ID, 
			&schedule.Shift.Name, 
			&startTime, 
			&endTime,
			&schedule.User.ID, 
			&schedule.User.Username, // Scan field user
			&schedule.User.Email,
			&schedule.User.FirstName,
			&schedule.User.LastName,
		)
		if scanErr != nil {
			zlog.Warn().Err(scanErr).Msg("Error scanning all schedules row (paginated)")
			err = fmt.Errorf("error scanning schedule row: %w", scanErr)
			return
		}
		schedule.Date = scheduleDate.Format(dateLayout)
		schedule.Shift.StartTime = startTime
		schedule.Shift.EndTime = endTime
		schedules = append(schedules, schedule)
	}
	if err = rows.Err(); err != nil {
		err = fmt.Errorf("error iterating all schedule rows: %w", err)
		return
	}
	return
}

func (r *scheduleRepo) DeleteSchedule(ctx context.Context, id int) error {
	query := "DELETE FROM user_schedules WHERE id = $1"
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		zlog.Error().Err(err).Int("schedule_id", id).Msg("Error deleting schedule")
		return fmt.Errorf("error deleting schedule %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows // Schedule tidak ditemukan
	}
	return nil
}

func (r *scheduleRepo) UpdateSchedule(ctx context.Context, schedule *models.UserSchedule) error {
	// --- Validasi tanggal sebelum query (jika formatnya string) ---
	scheduleDate, err := time.Parse(dateLayout, schedule.Date)
	if err != nil {
		// Kembalikan error format tanggal yang jelas
		return fmt.Errorf("invalid date format for schedule update, use YYYY-MM-DD: %w", err)
	}
	// --- Akhir Validasi Tanggal ---

	query := `UPDATE user_schedules SET user_id = $1, shift_id = $2, date = $3 WHERE id = $4`
	// --- TAMBAHKAN tag ---
	tag, err := r.db.Exec(ctx, query, schedule.UserID, schedule.ShiftID, scheduleDate, schedule.ID) // Gunakan scheduleDate
	// --- AKHIR TAMBAHKAN ---
	if err != nil {
		// Handle unique constraint (user_id, date)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			zlog.Warn().Err(err).Int("schedule_id", schedule.ID).Int("user_id", schedule.UserID).Str("date", schedule.Date).Msg("Unique constraint violation on schedule update")
			return fmt.Errorf("user %d already has a schedule on %s", schedule.UserID, schedule.Date)
		}
		// Handle foreign key constraint (user_id atau shift_id tidak valid)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23503" {
			zlog.Warn().Err(err).Int("schedule_id", schedule.ID).Int("user_id", schedule.UserID).Int("shift_id", schedule.ShiftID).Msg("Foreign key violation on schedule update")
			return fmt.Errorf("invalid user_id (%d) or shift_id (%d)", schedule.UserID, schedule.ShiftID)
		}
		// Error umum
		zlog.Error().Err(err).Int("schedule_id", schedule.ID).Msg("Error updating schedule")
		return fmt.Errorf("error updating schedule %d: %w", schedule.ID, err)
	}
	// --- TAMBAHKAN CEK RowsAffected ---
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows // Schedule tidak ditemukan
	}
	// --- AKHIR TAMBAHAN ---
	return nil
}
