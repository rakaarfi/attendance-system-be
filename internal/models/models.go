package models

import (
	"time"
)

type Role struct {
	ID   int    `json:"id"`
	Name string `json:"name" validate:"required,min=3,max=50"`
}

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username" validate:"required,min=3,max=100"`
	Password  string    `json:"-"`
	Email     string    `json:"email" validate:"required,email"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	RoleID    int       `json:"role_id" validate:"required"`
	Role      *Role     `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// Input struct terpisah untuk registrasi dan login
type RegisterUserInput struct {
	Username  string `json:"username" validate:"required,min=3,max=100"`
	Password  string `json:"password" validate:"required,min=6"`
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	RoleID    int    `json:"role_id" validate:"required,gt=0"`
}

type LoginUserInput struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type Shift struct {
	ID        int       `json:"id"`
	Name      string    `json:"name" validate:"required,min=3,max=100"`
	StartTime string    `json:"start_time" validate:"required"` // Format HH:MM:SS
	EndTime   string    `json:"end_time" validate:"required"`   // Format HH:MM:SS
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

type UserSchedule struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id" validate:"required"`
	ShiftID   int       `json:"shift_id" validate:"required"`
	Date      string    `json:"date" validate:"required"` // Format YYYY-MM-DD
	CreatedAt time.Time `json:"created_at"`
	User      *User     `json:"user,omitempty"`
	Shift     *Shift    `json:"shift,omitempty"`
}

type Attendance struct {
	ID         int        `json:"id"`
	UserID     int        `json:"user_id" validate:"required"`
	CheckInAt  time.Time  `json:"check_in_at"`
	CheckOutAt *time.Time `json:"check_out_at,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	User       *User      `json:"user,omitempty"`
}

type CheckInInput struct {
	Notes *string `json:"notes,omitempty"`
}

type CheckOutInput struct {
	Notes *string `json:"notes,omitempty"`
}

// Response standar untuk API
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type AdminUpdateUserInput struct {
	Username  string `json:"username" validate:"required,min=3,max=100"`
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	RoleID    int    `json:"role_id" validate:"required,gt=0"` // Pastikan role ID > 0
    // TIDAK ADA Password di sini. Buat endpoint terpisah jika Admin bisa reset password.
}

type UpdateProfileInput struct {
	Username  string `json:"username" validate:"required,min=3,max=100"`
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type UpdatePasswordInput struct {
	OldPassword string `json:"old_password" validate:"required,min=6"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}