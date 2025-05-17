package services

import (
	"errors"
	"fmt"
	"os"

	"github.com/adampresley/ownmyphotos/pkg/models"
)

type PhotoCacher interface {
	/*
	 * Checks if the thumbnail cache for the given photo exists.
	 */
	Exists(settings *models.Settings, photo *models.Photo) bool

	/*
	 * Returns the full path to the thumbnail cache for the given photo.
	 */
	GetFullCachePath(settings *models.Settings, photo *models.Photo) string

	/*
	 * Deletes the thumbnail cache for the given photo.
	 */
	Remove(settings *models.Settings, photo *models.Photo) error
}

type PhotoCacheConfig struct {
	CachePath string
}

type PhotoCache struct {
	cachePath string
}

func NewPhotoCache(config PhotoCacheConfig) PhotoCache {
	return PhotoCache{
		cachePath: config.CachePath,
	}
}

/*
Checks if the thumbnail cache for the given photo exists.
*/
func (c PhotoCache) Exists(settings *models.Settings, photo *models.Photo) bool {
	var (
		err error
	)

	fullPath := c.GetFullCachePath(settings, photo)

	if _, err = os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
		return true
	}

	return false
}

/*
Returns the full path to the thumbnail cache for the given photo.
*/
func (c PhotoCache) GetFullCachePath(settings *models.Settings, photo *models.Photo) string {
	albumPath := photo.GetAlbumPath(settings.LibraryPath)
	fullPath := GetThumbnailCachePath(settings.LibraryPath, c.cachePath, albumPath, photo.FileName, photo.Ext)
	return fullPath
}

func (c PhotoCache) Remove(settings *models.Settings, photo *models.Photo) error {
	var (
		err error
	)

	fullPath := GetThumbnailCachePath(
		settings.LibraryPath,
		c.cachePath,
		"",
		photo.FileName,
		photo.Ext,
	)

	if _, err = os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
		if err = os.Remove(fullPath); err != nil {
			return fmt.Errorf("error removing thumbnail cache for photo %s: %w", photo.ID, err)
		}

		return nil
	}

	return fmt.Errorf("thumbnail cache for photo %s not found: %w", photo.ID, err)
}
