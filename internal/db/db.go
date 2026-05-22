package db

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

type DBConfig struct {
	Driver      string
	SQLitePath  string
	DatabaseURL string
	Host        string
	Port        string
	User        string
	Password    string
	DBName      string
	SSLMode     string
	Timezone    string
}

func ConnectDB() (*gorm.DB, error) {
	config, err := LoadDBConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load database configuration: %w", err)
	}

	return ConnectConfiguredDB(config)
}

func ConnectConfiguredDB(config *DBConfig) (*gorm.DB, error) {
	if config == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	switch config.Driver {
	case DriverSQLite:
		if err := ensureParentDir(config.SQLitePath); err != nil {
			return nil, err
		}
		db, err := gorm.Open(sqlite.Open(config.SQLitePath), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
		}
		return db, nil
	case DriverPostgres:
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s timezone=%s", config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode, config.Timezone)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to postgres database: %w", err)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", config.Driver)
	}
}

func LoadDBConfig() (*DBConfig, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
	if driver == "" {
		driver = DriverSQLite
	}

	config := &DBConfig{Driver: driver}
	switch driver {
	case DriverSQLite:
		dbPath := strings.TrimSpace(os.Getenv("DB_PATH"))
		if dbPath == "" || strings.EqualFold(dbPath, "auto") {
			defaultPath, err := DefaultSQLitePath()
			if err != nil {
				return nil, err
			}
			dbPath = defaultPath
		}
		config.SQLitePath = dbPath
	case DriverPostgres:
		config.DatabaseURL = os.Getenv("DATABASE_URL")
		config.Host = os.Getenv("DB_HOST")
		config.Port = os.Getenv("DB_PORT")
		config.User = os.Getenv("DB_USER")
		config.Password = os.Getenv("DB_PASSWORD")
		config.DBName = os.Getenv("DB_NAME")
		config.SSLMode = os.Getenv("DB_SSLMODE")
		config.Timezone = os.Getenv("DB_TIMEZONE")
		if config.Host == "" || config.Port == "" || config.User == "" || config.Password == "" || config.DBName == "" || config.SSLMode == "" || config.Timezone == "" {
			return nil, fmt.Errorf("missing required postgres database configuration")
		}
		if config.DatabaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required when DB_DRIVER=postgres")
		}
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", driver)
	}

	return config, nil
}

func DefaultSQLitePath() (string, error) {
	base := ""
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve user home: %w", err)
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "SmarterOLJ", "smarter-olj.db"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "SmarterOLJ", "smarter-olj.db"), nil
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve user home: %w", err)
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "smarter-olj", "smarter-olj.db"), nil
	}
}

func ensureParentDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("sqlite database path is required")
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite database directory: %w", err)
	}
	return nil
}
