package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rakaarfi/attendance-system-be/internal/models"
	zlog "github.com/rs/zerolog/log"
)

type roleRepo struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) RoleRepository {
	return &roleRepo{db: db}
}

func (r *roleRepo) GetRoleByID(ctx context.Context, id int) (*models.Role, error) {
	query := `SELECT id, name FROM roles WHERE id = $1`
	role := &models.Role{}
	err := r.db.QueryRow(ctx, query, id).Scan(&role.ID, &role.Name)
	if err != nil {
		// Handle pgx.ErrNoRows
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows // Kembalikan error asli
		}
		zlog.Error().Err(err).Int("role_id", id).Msg("Error getting role by ID")
		return nil, fmt.Errorf("error getting role by id %d: %w", id, err)
	}
	return role, nil
}

func (r *roleRepo) CreateRole(ctx context.Context, role *models.Role) (int, error) {
	query := `INSERT INTO roles (name) VALUES ($1) RETURNING id`
	var roleID int
	err := r.db.QueryRow(ctx, query, role.Name).Scan(&roleID)
	if err != nil {
		// Handle unique constraint violation (name)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			zlog.Warn().Err(err).Str("role_name", role.Name).Msg("Role name already exists")
			return 0, fmt.Errorf("role name '%s' already exists", role.Name)
		}
		// Error umum
		zlog.Error().Err(err).Str("role_name", role.Name).Msg("Error creating role")
		return 0, fmt.Errorf("error creating role: %w", err)
	}
	return roleID, nil
}

func (r *roleRepo) GetAllRoles(ctx context.Context) ([]models.Role, error) {
	query := `SELECT id, name FROM roles ORDER BY name`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		zlog.Error().Err(err).Msg("Error getting all roles")
		return nil, fmt.Errorf("error getting all roles: %w", err)
	}
	defer rows.Close()

	roles := []models.Role{}
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.Name); err != nil {
			zlog.Warn().Err(err).Msg("Error scanning role row")
			continue // Lanjutkan ke baris berikutnya
		}
		roles = append(roles, role)
	}

	if err = rows.Err(); err != nil {
		zlog.Error().Err(err).Msg("Error iterating role rows")
		return nil, fmt.Errorf("error iterating role rows: %w", err)
	}
	return roles, nil
}

func (r *roleRepo) UpdateRole(ctx context.Context, role *models.Role) error {
	query := `UPDATE roles SET name = $1 WHERE id = $2`
	tag, err := r.db.Exec(ctx, query, role.Name, role.ID)
	if err != nil {
		// Handle unique constraint violation (name)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			zlog.Warn().Err(err).Str("role_name", role.Name).Int("role_id", role.ID).Msg("Role name already exists on update")
			return fmt.Errorf("role name '%s' already exists", role.Name)
		}
		// Error umum
		zlog.Error().Err(err).Int("role_id", role.ID).Msg("Error updating role")
		return fmt.Errorf("error updating role %d: %w", role.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows // Role tidak ditemukan
	}
	return nil
}

func (r *roleRepo) DeleteRole(ctx context.Context, id int) error {
	// PENTING: Cek dulu apakah ada user yang masih menggunakan role ini
	countQuery := `SELECT COUNT(*) FROM users WHERE role_id = $1`
	var userCount int
	err := r.db.QueryRow(ctx, countQuery, id).Scan(&userCount)
	if err != nil {
		zlog.Error().Err(err).Int("role_id", id).Msg("Error checking users for role before deletion")
		return fmt.Errorf("error checking users for role %d: %w", id, err)
	}

	if userCount > 0 {
		zlog.Warn().Int("role_id", id).Int("user_count", userCount).Msg("Attempted to delete role that is still in use")
		return fmt.Errorf("cannot delete role: %d user(s) still assigned to this role", userCount)
	}

	// Jika tidak ada user, lanjutkan penghapusan
	deleteQuery := `DELETE FROM roles WHERE id = $1`
	tag, err := r.db.Exec(ctx, deleteQuery, id)
	if err != nil {
		// Error saat delete (seharusnya jarang terjadi jika pengecekan user berhasil)
		zlog.Error().Err(err).Int("role_id", id).Msg("Error deleting role")
		return fmt.Errorf("error deleting role %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows // Role tidak ditemukan
	}
	return nil
}
