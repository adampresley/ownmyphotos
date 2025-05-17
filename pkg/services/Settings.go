package services

import (
	"fmt"

	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/rfberaldo/sqlz"
)

type SettingsServicer interface {
	Read() (*models.Settings, error)
	Save(settings *models.Settings) error
}

type SettingsServiceConfig struct {
	DB *sqlz.DB
}

type SettingsService struct {
	db *sqlz.DB
}

func NewSettingsService(config SettingsServiceConfig) SettingsService {
	return SettingsService{
		db: config.DB,
	}
}

func (s SettingsService) Read() (*models.Settings, error) {
	var (
		err error
	)

	result := &models.Settings{
		ThumbnailSize:     300,
		MaxWorkers:        5,
		CollectorSchedule: "0 */1 * * *",
	}

	sql := `
SELECT
   id
	, collector_schedule
	, max_workers
   , library_path
	, thumbnail_size
FROM settings
WHERE 1=1
   AND id=1
   `

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.QueryRow(ctx, result, sql); err != nil && !sqlz.IsNotFound(err) {
		return result, fmt.Errorf("error querying for settings: %w", err)
	}

	return result, nil
}

func (s SettingsService) Save(settings *models.Settings) error {
	var (
		err error
	)

	sql := `
INSERT INTO settings (
   id
	, collector_schedule
	, max_workers
   , library_path
	, thumbnail_size
) VALUES (
   1
	, ?
	, ?
   , ?
	, ?
)
ON CONFLICT (id) DO
UPDATE SET
	collector_schedule=excluded.collector_schedule
	, max_workers=excluded.max_workers
   , library_path=excluded.library_path
	, thumbnail_size=excluded.thumbnail_size
   `

	args := []any{
		settings.CollectorSchedule,
		settings.MaxWorkers,
		settings.LibraryPath,
		settings.ThumbnailSize,
	}

	ctx, cancel := DBContext()
	defer cancel()

	if _, err = s.db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	return nil
}
