package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	zlog "github.com/rs/zerolog/log"
)

type attendanceRepo struct {
	db *pgxpool.Pool
}

func NewAttendanceRepository(db *pgxpool.Pool) AttendanceRepository {
	return &attendanceRepo{db: db}
}

// CreateCheckIn records a check-in event
func (r *attendanceRepo) CreateCheckIn(ctx context.Context, userID int, checkInTime time.Time, notes *string) (int, error) {
	query := `INSERT INTO attendances (user_id, check_in_at, notes) VALUES ($1, $2, $3) RETURNING id`
	var attendanceID int
	err := r.db.QueryRow(ctx, query, userID, checkInTime, notes).Scan(&attendanceID)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Time("check_in_at", checkInTime).Msg("Error creating check-in for user")
		return 0, fmt.Errorf("error creating check-in for user %d: %w", userID, err)
	}
	zlog.Info().Int("attendance_id", attendanceID).Int("user_id", userID).Time("check_in_at", checkInTime).Msg("Check-in created successfully")
	return attendanceID, nil
}

// GetLastAttendance retrieves the most recent attendance record for a user
// Useful for checking status (already checked in?) or finding record to checkout.
func (r *attendanceRepo) GetLastAttendance(ctx context.Context, userID int) (*models.Attendance, error) {
	query := `
        SELECT id, user_id, check_in_at, check_out_at, notes, created_at, updated_at
        FROM attendances
        WHERE user_id = $1
        ORDER BY check_in_at DESC
        LIMIT 1`
	att := &models.Attendance{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&att.ID,
		&att.UserID,
		&att.CheckInAt,
		&att.CheckOutAt, // Handles NULL automatically with *time.Time
		&att.Notes,      // Handles NULL automatically with *string
		&att.CreatedAt,
		&att.UpdatedAt,
	)
	if err != nil {
		// Penting: ErrNoRows di sini berarti user belum pernah absensi sama sekali
		if errors.Is(err, pgx.ErrNoRows) {
			zlog.Warn().Int("user_id", userID).Msg("User has no attendance record")
			return nil, pgx.ErrNoRows // Kembalikan error asli agar handler bisa bedakan
		}
		zlog.Error().Err(err).Int("user_id", userID).Msg("Error getting last attendance for user")
		return nil, fmt.Errorf("error getting last attendance for user %d: %w", userID, err)
	}
	return att, nil
}

// UpdateCheckOut records the check-out time for a specific attendance record
func (r *attendanceRepo) UpdateCheckOut(ctx context.Context, attendanceID int, checkOutTime time.Time, notes *string) error {
	// Update notes jika disediakan, jika tidak, biarkan notes yang ada
	query := `UPDATE attendances SET check_out_at = $1, updated_at = CURRENT_TIMESTAMP, notes = COALESCE($2, notes)
              WHERE id = $3 AND check_out_at IS NULL` // Pastikan hanya update yang belum checkout

	tag, err := r.db.Exec(ctx, query, checkOutTime, notes, attendanceID)
	if err != nil {
		zlog.Error().Err(err).Int("attendance_id", attendanceID).Msg("Error updating check-out for attendance ID")
		return fmt.Errorf("error updating check-out for attendance id %d: %w", attendanceID, err)
	}
	if tag.RowsAffected() == 0 {
		// Ini bisa berarti ID tidak ditemukan ATAU sudah checkout sebelumnya
		zlog.Warn().Int("attendance_id", attendanceID).Msg("Attendance record not found or already checked out")
		return fmt.Errorf("attendance record %d not found or already checked out", attendanceID)
	}
	return nil
}

// GetAttendancesByUser retrieves attendance records for a user within a date range
func (r *attendanceRepo) GetAttendancesByUser(ctx context.Context, userID int, startDate, endDate time.Time, page, limit int) (attendances []models.Attendance, totalCount int, err error) {
	// --- 1. Count Total ---
	// Gunakan >= startDate dan <= endDate karena handler akan set endDate ke akhir hari
	countQuery := `SELECT COUNT(*) FROM attendances WHERE user_id = $1 AND check_in_at >= $2 AND check_in_at <= $3`
	err = r.db.QueryRow(ctx, countQuery, userID, startDate, endDate).Scan(&totalCount)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Time("start", startDate).Time("end", endDate).Msg("Error counting user attendances")
		err = fmt.Errorf("error counting attendances for user %d: %w", userID, err)
		return // Kembalikan error
	}
	if totalCount == 0 {
		attendances = []models.Attendance{} // Return slice kosong
		return
	}

	// --- 2. Calculate Offset ---
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// --- 3. Query Data ---
	query := `
        SELECT id, user_id, check_in_at, check_out_at, notes, created_at, updated_at
        FROM attendances
        WHERE user_id = $1 AND check_in_at >= $2 AND check_in_at <= $3
        ORDER BY check_in_at DESC -- Order by check_in paling baru
        LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, query, userID, startDate, endDate, limit, offset)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Msg("Error querying paginated user attendances")
		err = fmt.Errorf("error getting paginated attendances for user %d: %w", userID, err)
		return
	}
	defer rows.Close()

	// --- 4. Scan Results ---
	attendances = []models.Attendance{}
	for rows.Next() {
		var att models.Attendance
		scanErr := rows.Scan(
			&att.ID,
			&att.UserID,
			&att.CheckInAt,
			&att.CheckOutAt, // Handles NULL
			&att.Notes,      // Handles NULL
			&att.CreatedAt,
			&att.UpdatedAt,
		)
		if scanErr != nil {
			zlog.Warn().Err(scanErr).Int("user_id", userID).Msg("Error scanning user attendance row (paginated)")
			err = fmt.Errorf("error scanning attendance row: %w", scanErr)
			return // Return error jika scan gagal
		}
		attendances = append(attendances, att)
	}
	if err = rows.Err(); err != nil {
		zlog.Error().Err(err).Int("user_id", userID).Msg("Error iterating user attendance rows")
		err = fmt.Errorf("error iterating attendance rows: %w", err)
		return
	}

	return // attendances, totalCount, nil error
}

// GetAllAttendances retrieves all attendance records within a date range (for Admin)
// Includes user information
func (r *attendanceRepo) GetAllAttendances(ctx context.Context, startDate, endDate time.Time, page, limit int) (attendances []models.Attendance, totalCount int, err error) {
	// --- 1. Count Total (tanpa join) ---
	countQuery := `SELECT COUNT(*) FROM attendances WHERE check_in_at >= $1 AND check_in_at <= $2`
	err = r.db.QueryRow(ctx, countQuery, startDate, endDate).Scan(&totalCount)
	if err != nil {
		zlog.Error().Err(err).Time("start", startDate).Time("end", endDate).Msg("Error counting all attendances")
		err = fmt.Errorf("error counting all attendances: %w", err)
		return
	}
	if totalCount == 0 {
		attendances = []models.Attendance{}
		return
	}

	// --- 2. Calculate Offset ---
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// --- 3. Query Data (dengan join user) ---
	query := `
        SELECT a.id, a.user_id, a.check_in_at, a.check_out_at, a.notes, a.created_at, a.updated_at,
               u.id as userid, u.username, u.first_name, u.last_name, u.email
        FROM attendances a
        JOIN users u ON a.user_id = u.id
        WHERE a.check_in_at >= $1 AND a.check_in_at <= $2
        ORDER BY a.check_in_at DESC, u.username ASC -- Order by check_in, lalu username
        LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, query, startDate, endDate, limit, offset)
	if err != nil {
		zlog.Error().Err(err).Msg("Error querying paginated all attendances report")
		err = fmt.Errorf("error getting paginated all attendances report: %w", err)
		return
	}
	defer rows.Close()

	// --- 4. Scan Results ---
	attendances = []models.Attendance{}
	for rows.Next() {
		var att models.Attendance
		att.User = &models.User{} // !!! Penting: Inisialisasi User sebelum scan !!!
		scanErr := rows.Scan(
			&att.ID, &att.UserID, &att.CheckInAt, &att.CheckOutAt, &att.Notes,
			&att.CreatedAt, &att.UpdatedAt,
			&att.User.ID, &att.User.Username, &att.User.FirstName, &att.User.LastName, &att.User.Email,
		)
		if scanErr != nil {
			zlog.Warn().Err(scanErr).Msg("Error scanning attendance report row (paginated)")
			err = fmt.Errorf("error scanning attendance report row: %w", scanErr)
			return
		}
		attendances = append(attendances, att)
	}
	if err = rows.Err(); err != nil {
		zlog.Error().Err(err).Msg("Error iterating attendance report rows")
		err = fmt.Errorf("error iterating attendance report rows: %w", err)
		return
	}

	return // attendances, totalCount, nil error
}
