package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/rakaarfi/attendance-system-be/internal/models"
)

// MockShiftRepository mocks the ShiftRepository interface.
type MockShiftRepository struct {
	mock.Mock
}

func (m *MockShiftRepository) CreateShift(ctx context.Context, shift *models.Shift) (int, error) {
	args := m.Called(ctx, shift)
	return args.Int(0), args.Error(1)
}

func (m *MockShiftRepository) GetShiftByID(ctx context.Context, id int) (*models.Shift, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Shift), args.Error(1)
}

func (m *MockShiftRepository) GetAllShifts(ctx context.Context) ([]models.Shift, error) {
	args := m.Called(ctx)
    // Handle potentially nil slice return
    ret := args.Get(0)
    if ret == nil {
        // Return nil slice explicitly if needed, otherwise let testify handle based on setup
        return nil, args.Error(1)
    }
	return ret.([]models.Shift), args.Error(1)
}

func (m *MockShiftRepository) UpdateShift(ctx context.Context, shift *models.Shift) error {
	args := m.Called(ctx, shift)
	return args.Error(0)
}

func (m *MockShiftRepository) DeleteShift(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}