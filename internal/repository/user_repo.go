package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	zlog "github.com/rs/zerolog/log"
)

type userRepo struct {
	db *pgxpool.Pool
}

// NewUserRepository membuat instance baru dari UserRepository
func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) CreateUser(ctx context.Context, input *models.RegisterUserInput, hashedPassword string) (int, error) {
	query := `INSERT INTO users (username, password, email, first_name, last_name, role_id)
              VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	var userID int
	err := r.db.QueryRow(ctx, query,
		input.Username,
		hashedPassword,
		input.Email,
		input.FirstName,
		input.LastName,
		input.RoleID,
	).Scan(&userID)

	if err != nil {
		zlog.Error().Err(err).Str("username", input.Username).Msg("Error creating user")
		// Handle potential unique constraint violation error pgx.PgError code 23505
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			zlog.Warn().Err(err).Str("username", input.Username).Msg("Username already taken")
			return 0, fmt.Errorf("username already taken: %w", err)
		}
		return 0, fmt.Errorf("error creating user: %w", err)
	}
	zlog.Info().Int("user_id", userID).Str("username", input.Username).Msg("User created successfully")
	return userID, nil
}

func (r *userRepo) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT u.id, u.username, u.password, u.email, u.first_name, u.last_name, u.role_id, u.created_at, u.updated_at,
	                 r.id as roleid, r.name as rolename
	          FROM users u
	          JOIN roles r ON u.role_id = r.id
	          WHERE u.username = $1`
	user := &models.User{Role: &models.Role{}} // Inisialisasi Role
	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.RoleID,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role.ID,   // Scan ke field Role
		&user.Role.Name, // Scan ke field Role
	)
	if err != nil {
		// Handle pgx.ErrNoRows jika user tidak ditemukan
		zlog.Error().Err(err).Str("username", username).Msg("Error getting user by username")
		return nil, fmt.Errorf("error getting user by username %s: %w", username, err)
	}
	zlog.Info().Str("username", username).Msg("User retrieved successfully")
	return user, nil
}

func (r *userRepo) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	query := `SELECT id, username, password, email, first_name, last_name, role_id, created_at, updated_at
	          FROM users WHERE id = $1`
	user := &models.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.RoleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		zlog.Error().Err(err).Int("user_id", id).Msg("Error getting user by id")
		return nil, fmt.Errorf("error getting user by id %d: %w", id, err)
	}
	zlog.Info().Int("user_id", id).Msg("User retrieved successfully")
	return user, nil
}

func (r *userRepo) DeleteUserByID(ctx context.Context, id int) error {
	query := `DELETE FROM users WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id)

	if err != nil {
		// Error umum saat eksekusi query
		return fmt.Errorf("error deleting user with id %d: %w", id, err)
	}

	// Cek apakah ada baris yang benar-benar terhapus
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// GetAllUsers retrieves a paginated list of users with role information.
func (r *userRepo) GetAllUsers(ctx context.Context, page, limit int) (users []models.User, totalCount int, err error) {
	// --- 1. Hitung Total User (Tanpa Pagination) ---
	countQuery := `SELECT COUNT(*) FROM users`
	err = r.db.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		zlog.Error().Err(err).Msg("Error counting total users")
		err = fmt.Errorf("error counting total users: %w", err)
		return // Kembalikan error
	}

	// Jika tidak ada user, kembalikan slice kosong dan total 0
	if totalCount == 0 {
		users = []models.User{} // Pastikan mengembalikan slice kosong, bukan nil
		return                  // Kembalikan users kosong, totalCount 0, err nil
	}

	// --- 2. Hitung Offset ---
	offset := (page - 1) * limit
	if offset < 0 { // Jaga-jaga jika page < 1 (seharusnya divalidasi di handler)
		offset = 0
	}

	// --- 3. Query Pengguna dengan Pagination dan Role ---
	query := `SELECT u.id, u.username, u.email, u.first_name, u.last_name, u.role_id, u.created_at, u.updated_at,
                     r.id as roleid, r.name as rolename
              FROM users u
              LEFT JOIN roles r ON u.role_id = r.id
              ORDER BY u.id ASC -- Atau u.username, ORDER BY penting untuk pagination stabil
              LIMIT $1 OFFSET $2` // Tambahkan LIMIT dan OFFSET

	rows, err := r.db.Query(ctx, query, limit, offset) // Pass limit dan offset sebagai parameter
	if err != nil {
		zlog.Error().Err(err).Msg("Error querying paginated users with roles")
		err = fmt.Errorf("error getting paginated users with roles: %w", err)
		return // Kembalikan error (totalCount mungkin sudah ada, tapi users belum)
	}
	defer rows.Close()

	// --- 4. Scan Hasil ---
	users = []models.User{} // Inisialisasi slice
	for rows.Next() {
		var user models.User
		user.Role = &models.Role{} // Inisialisasi pointer Role
		scanErr := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.FirstName, &user.LastName,
			&user.RoleID, &user.CreatedAt, &user.UpdatedAt,
			&user.Role.ID, &user.Role.Name,
		)
		if scanErr != nil {
			zlog.Warn().Err(scanErr).Msg("Error scanning user row with role (paginated)")
			// Mungkin lanjutkan saja, atau hentikan dan kembalikan error?
			// Jika ada error scan, mungkin lebih baik hentikan.
			err = fmt.Errorf("error scanning user row: %w", scanErr)
			return // Kembalikan users yang sudah terkumpul sejauh ini & error
		}
		users = append(users, user)
	}

	// Cek error setelah loop selesai
	if err = rows.Err(); err != nil {
		zlog.Error().Err(err).Msg("Error iterating paginated user rows with roles")
		err = fmt.Errorf("error iterating paginated user rows: %w", err)
		return // Kembalikan users yang sudah terkumpul & error
	}

	// Jika sampai sini tanpa error, kembalikan users, totalCount, dan err nil
	return users, totalCount, nil
}

func (r *userRepo) UpdateUserByID(ctx context.Context, id int, input *models.AdminUpdateUserInput) error {
	query := `UPDATE users SET username = $1, email = $2, first_name = $3, last_name = $4, role_id = $5
              WHERE id = $6` // updated_at dihandle trigger

	tag, err := r.db.Exec(ctx, query, input.Username, input.Email, input.FirstName, input.LastName, input.RoleID, id)
	if err != nil {
		// Handle unique constraint (username/email exists)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			// Cek constraint name jika perlu untuk bedakan username/email
			fieldName := "username or email"
			if strings.Contains(pgErr.ConstraintName, "email") {
				fieldName = "email"
			}
			if strings.Contains(pgErr.ConstraintName, "username") {
				fieldName = "username"
			}

			zlog.Warn().Err(err).Int("user_id", id).Str("field", fieldName).Msg("Unique constraint violation on user update")
			return fmt.Errorf("%s already exists", fieldName) // Error spesifik
		}
		// Error umum
		zlog.Error().Err(err).Int("user_id", id).Msg("Error updating user")
		return fmt.Errorf("error updating user: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows // User tidak ditemukan
	}
	return nil
}

func (r *userRepo) UpdateUserPassword(ctx context.Context, id int, hashedPassword string) error {
	query := `UPDATE users SET password = $1 WHERE id = $2`
	
	tag, err := r.db.Exec(ctx, query, hashedPassword, id) // Simpan HASHED password
	if err != nil {
		zlog.Error().Err(err).Int("user_id", id).Msg("Error updating user password")
		return fmt.Errorf("error updating user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Seharusnya tidak terjadi jika ID dari JWT valid
		return pgx.ErrNoRows // User tidak ditemukan
	}
	return nil
}

func (r *userRepo) UpdateUserProfile(ctx context.Context, id int, input *models.UpdateProfileInput) error {
	// Hanya update field yang relevan untuk profil
	query := `UPDATE users SET username = $1, email = $2, first_name = $3, last_name = $4
              WHERE id = $5` // updated_at akan dihandle trigger

	tag, err := r.db.Exec(ctx, query, input.Username, input.Email, input.FirstName, input.LastName, id)
	if err != nil {
		// Handle unique constraint (username/email exists)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			fieldName := "username or email"
			if strings.Contains(pgErr.ConstraintName, "email") {
				fieldName = "email"
			}
			if strings.Contains(pgErr.ConstraintName, "username") {
				fieldName = "username"
			}
			zlog.Warn().Err(err).Int("user_id", id).Str("field", fieldName).Msg("Unique constraint violation on user profile update")
			return fmt.Errorf("%s already exists", fieldName) // Error spesifik
		}
		// Error umum
		zlog.Error().Err(err).Int("user_id", id).Msg("Error updating user profile")
		return fmt.Errorf("error updating user profile: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Ini seharusnya tidak terjadi jika ID diambil dari JWT yang valid,
		// tapi cek untuk keamanan tambahan.
		return pgx.ErrNoRows // User tidak ditemukan
	}
	return nil
}
