// internal/repository/mocks/user_repository_mock.go
package mocks

import (
	"context"

	"github.com/rakaarfi/attendance-system-be/internal/models"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository adalah mock untuk UserRepository
type MockUserRepository struct {
	mock.Mock
}

// Implementasikan SEMUA method dari interface UserRepository

func (m *MockUserRepository) CreateUser(ctx context.Context, user *models.RegisterUserInput, hashedPassword string) (int, error) {
	// Beritahu testify method ini dipanggil dengan argumen apa saja
	args := m.Called(ctx, user, hashedPassword)
	// Kembalikan apa yang sudah di-set di expectation (.Return(...))
	return args.Int(0), args.Error(1)
}

func (m *MockUserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	// Handle nil return jika user tidak ditemukan
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// --- Mock untuk RoleRepository ---
type MockRoleRepository struct {
	mock.Mock
}

func (m *MockRoleRepository) GetRoleByID(ctx context.Context, id int) (*models.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

// Buat mock serupa untuk ShiftRepository, ScheduleRepository, AttendanceRepository...
