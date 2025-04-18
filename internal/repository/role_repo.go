package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rakaarfi/attendance-system-be/internal/models"
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
		return nil, fmt.Errorf("error getting role by id %d: %w", id, err)
	}
	return role, nil
}
