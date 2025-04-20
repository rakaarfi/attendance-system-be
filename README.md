# attendance-system-be

## Description

`attendance-system-be` is the backend API service for an employee attendance management system. It provides functionalities for user authentication, role-based access control, managing shifts and schedules, and recording employee attendance. It includes separate endpoints for administrative tasks and regular user operations.

## Features

*   User Authentication (Login/Register)
*   Role-Based Access Control (Admin/User)
*   Shift Management (Create, Read, Update, Delete - Admin)
*   Schedule Management (Create, Read, Update, Delete - Admin)
*   Attendance Recording (Clock In/Clock Out - User)
*   User Management (View users - Admin)
*   Attendance Reporting (View attendance records - Admin/User)

## Prerequisites

*   Go (version 1.24.0 or higher)
*   PostgreSQL
*   `golang-migrate/migrate` CLI tool (for database migrations)

## Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/rakaarfi/attendance-system-be.git
    ```
2.  **Navigate to the project directory:**
    ```bash
    cd attendance-system-be
    ```
3.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

## Configuration

This project uses environment variables for configuration. You can set them directly in your environment or create a `.env` file in the project root.

1.  **Create a `.env` file** by copying the example below and placing it in the project root:

    ```dotenv
    # .env.example
    # Database Configuration
    DB_HOST=localhost
    DB_PORT=5432
    DB_USER=your_db_user
    DB_PASSWORD=your_db_password
    DB_NAME=attendance_db
    DB_SSLMODE=disable # or require, verify-full, etc.

    # Application Configuration
    APP_PORT=3000

    # JWT Configuration
    JWT_SECRET=your_strong_jwt_secret
    JWT_EXPIRATION_HOURS=24 # Example: Token valid for 24 hours

    # Logger Configuration (Optional - Defaults are usually fine)
    # LOG_LEVEL=info # (trace, debug, info, warn, error, fatal, panic)
    # LOG_FILE_PATH=./logs/app.log # Path to log file
    # LOG_MAX_SIZE_MB=100 # Max size in MB before rotation
    # LOG_MAX_BACKUPS=3 # Max number of old log files to keep
    # LOG_MAX_AGE_DAYS=7 # Max number of days to retain old log files
    # LOG_COMPRESS=false # Compress rotated log files
    ```

2.  **Important:** Replace the placeholder values (e.g., `your_db_user`, `your_db_password`, `attendance_db`, `your_strong_jwt_secret`) with your actual configuration details.

## Database Setup

1.  Ensure your PostgreSQL server is running and accessible using the credentials provided in your `.env` file.
2.  Create the database specified in `DB_NAME` if it doesn't already exist.
3.  Install the `golang-migrate/migrate` CLI tool if you haven't already:
    ```bash
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
    ```
4.  Apply the database migrations. Run the following command from the project root directory, replacing the placeholders with the values from your `.env` file:
    ```bash
    migrate -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}" -path migrations up
    ```
    *(Adjust `sslmode` based on your PostgreSQL server's requirements.)*

## Running the Application

1.  Start the API server:
    ```bash
    go run cmd/api/main.go
    ```
2.  The API will be available at `http://localhost:APP_PORT` (defaulting to port 3000 if `APP_PORT` is not set in your `.env` file).

## API Structure

The main API routes are versioned and available under the `/api/v1` path.

## Testing

Unit tests are planned for future development. Mock implementations for repositories can be found in `internal/repository/mocks/` to aid in writing tests. Currently, there are no automated tests to run.

## Project Structure

```
.
├── cmd/api/             # Main application entry point
├── configs/             # Configuration loading (.env)
├── internal/            # Core application logic
│   ├── api/             # API route definitions and handlers (v1, v2, etc.)
│   ├── database/        # Database connection setup (PostgreSQL)
│   ├── logger/          # Logging setup (Zerolog, Lumberjack)
│   ├── middleware/      # Request middleware (auth, logging, etc.)
│   ├── models/          # Data structure definitions (structs)
│   ├── repository/      # Database interaction logic (data access layer)
│   │   └── mocks/       # Mock implementations for testing
│   └── utils/           # Utility functions (hashing, JWT, pagination, etc.)
├── migrations/          # Database migration files (.sql)
├── .env.example         # Example environment file (to be created based on README)
├── .gitignore           # Git ignore rules
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
└── README.md            # This file
