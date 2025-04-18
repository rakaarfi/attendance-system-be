package mocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/rakaarfi/attendance-system-be/internal/models"
)

// MockScheduleRepository mocks the ScheduleRepository interface.
type MockScheduleRepository struct {
	mock.Mock
}

func (m *MockScheduleRepository) CreateSchedule(ctx context.Context, schedule *models.UserSchedule) (int, error) {
	args := m.Called(ctx, schedule)
	return args.Int(0), args.Error(1)
}

func (m *MockScheduleRepository) GetScheduleByUserAndDate(ctx context.Context, userID int, date time.Time) (*models.UserSchedule, error) {
	args := m.Called(ctx, userID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserSchedule), args.Error(1)
}

func (m *MockScheduleRepository) GetSchedulesByUser(ctx context.Context, userID int, startDate, endDate time.Time) ([]models.UserSchedule, error) {
	args := m.Called(ctx, userID, startDate, endDate)
    ret := args.Get(0)
    if ret == nil {
        return nil, args.Error(1)
    }
	return ret.([]models.UserSchedule), args.Error(1)
}