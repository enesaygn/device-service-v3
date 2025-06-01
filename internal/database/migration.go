// internal/database/migration.go
package database

import (
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"

	"device-service/internal/config"
)

// Migrator handles database migrations
type Migrator struct {
	db     *DB
	logger *zap.Logger
	config *config.DatabaseConfig
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *DB, logger *zap.Logger, config *config.DatabaseConfig) *Migrator {
	return &Migrator{
		db:     db,
		logger: logger,
		config: config,
	}
}

// Up runs all up migrations
func (m *Migrator) Up() error {
	migrator, err := m.createMigrator()
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}

	m.logger.Info("Database migrations completed successfully")
	return nil
}

// Down runs all down migrations
func (m *Migrator) Down() error {
	migrator, err := m.createMigrator()
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer migrator.Close()

	if err := migrator.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration down failed: %w", err)
	}

	m.logger.Info("Database migrations rolled back successfully")
	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	migrator, err := m.createMigrator()
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrator: %w", err)
	}
	defer migrator.Close()

	version, dirty, err := migrator.Version()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get version: %w", err)
	}

	return version, dirty, nil
}

// Force forces a specific migration version
func (m *Migrator) Force(version int) error {
	migrator, err := m.createMigrator()
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer migrator.Close()

	if err := migrator.Force(version); err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}

	m.logger.Info("Migration version forced", zap.Int("version", version))
	return nil
}

// createMigrator creates a migrate instance
func (m *Migrator) createMigrator() (*migrate.Migrate, error) {
	driver, err := postgres.WithInstance(m.db.DB, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Get absolute path to migrations
	migrationsPath, err := filepath.Abs("migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to get migrations path: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	migrator, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return migrator, nil
}

// RunCleanup runs the cleanup function to remove old records
func (m *Migrator) RunCleanup() error {
	_, err := m.db.Exec("SELECT cleanup_old_records()")
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	m.logger.Info("Database cleanup completed")
	return nil
}
