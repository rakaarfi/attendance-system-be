package mocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/rakaarfi/attendance-system-be/internal/models"
)

// MockAttendanceRepository mocks the AttendanceRepository interface.
type MockAttendanceRepository struct {
	mock.Mock
}

func (m *MockAttendanceRepository) CreateCheckIn(ctx context.Context, userID int, checkInTime time.Time, notes *string) (int, error) {
	// Use mock.Anything for time.Time if precise matching is hard/unnecessary
	args := m.Called(ctx, userID, mock.AnythingOfType("time.Time"), notes)
	return args.Int(0), args.Error(1)
}

func (m *MockAttendanceRepository) GetLastAttendance(ctx context.Context, userID int) (*models.Attendance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Attendance), args.Error(1)
}

func (m *MockAttendanceRepository) UpdateCheckOut(ctx context.Context, attendanceID int, checkOutTime time.Time, notes *string) error {
	// Use mock.Anything for time.Time
	args := m.Called(ctx, attendanceID, mock.AnythingOfType("time.Time"), notes)
	return args.Error(0)
}

func (m *MockAttendanceRepository) GetAttendancesByUser(ctx context.Context, userID int, startDate, endDate time.Time) ([]models.Attendance, error) {
	// Use mock.Anything for time boundaries if needed
	args := m.Called(ctx, userID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"))
	ret := args.Get(0)
	if ret == nil {
		return nil, args.Error(1)
	}
	return ret.([]models.Attendance), args.Error(1)
}

func (m *MockAttendanceRepository) GetAllAttendances(ctx context.Context, startDate, endDate time.Time) ([]models.Attendance, error) {
	// Use mock.Anything for time boundaries
	args := m.Called(ctx, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"))
	ret := args.Get(0)
	if ret == nil {
		return nil, args.Error(1)
	}
	return ret.([]models.Attendance), args.Error(1)
}
