-- Migrations Down

DROP TRIGGER IF EXISTS set_timestamp_attendances ON attendances;
DROP TRIGGER IF EXISTS set_timestamp_shifts ON shifts;
DROP TRIGGER IF EXISTS set_timestamp_users ON users;

DROP FUNCTION IF EXISTS trigger_set_timestamp();

DROP TABLE IF EXISTS attendances;
DROP TABLE IF EXISTS user_schedules;
DROP TABLE IF EXISTS shifts;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;