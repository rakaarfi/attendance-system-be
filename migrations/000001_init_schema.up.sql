-- Migrations Up

-- Tabel Peran
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

-- Tabel Pengguna
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    role_id INT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id)
);

-- Tabel Shift Kerja
CREATE TABLE shifts (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Tabel Penjadwalan Shift per User per Hari
CREATE TABLE user_schedules (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    shift_id INT NOT NULL,
    date DATE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (shift_id) REFERENCES shifts(id) ON DELETE RESTRICT,
    UNIQUE (user_id, date) -- Mencegah jadwal ganda per hari
);

-- Tabel Log Kehadiran Aktual
CREATE TABLE attendances (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    check_in_at TIMESTAMPTZ NOT NULL,
    check_out_at TIMESTAMPTZ NULL, -- HARUS NULLABLE
    notes TEXT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Function to update updated_at column automatically
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for users table
CREATE TRIGGER set_timestamp_users
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();

-- Trigger for shifts table
CREATE TRIGGER set_timestamp_shifts
BEFORE UPDATE ON shifts
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();

-- Trigger for attendances table
CREATE TRIGGER set_timestamp_attendances
BEFORE UPDATE ON attendances
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();

-- Tambahkan index untuk performa
CREATE INDEX idx_attendances_user_checkin ON attendances(user_id, check_in_at);
CREATE INDEX idx_user_schedules_date ON user_schedules(date);
CREATE INDEX idx_users_role_id ON users(role_id);

-- Seed data Roles (contoh)
INSERT INTO roles (name) VALUES ('Admin'), ('Employee');