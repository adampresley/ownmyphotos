package cache

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
	"github.com/alitto/pond/v2"
	"github.com/nfnt/resize"
)

type CacheCreator interface {
	CreateCache()
}

type CacheCreatorConfig struct {
	SettingsService services.SettingsServicer
	CacheDirectory  string
	MaxCacheWorkers int
}

type CacheCreatorService struct {
	settingsService services.SettingsServicer
	cacheDirectory  string
	maxCacheWorkers int

	thumbnailSize    uint
	libraryDirectory string
}

func NewCacheCreatorService(config CacheCreatorConfig) CacheCreatorService {
	return CacheCreatorService{
		settingsService: config.SettingsService,
		cacheDirectory:  config.CacheDirectory,
		maxCacheWorkers: config.MaxCacheWorkers,
	}
}

func (c CacheCreatorService) CreateCache() {
	var (
		err      error
		settings *models.Settings
		validExt = map[string]bool{
			".jpg":  true,
			".jpeg": true,
		}
	)

	if settings, err = c.settingsService.Read(); err != nil {
		slog.Error("error getting library information from the database", "error", err)
		return
	}

	if settings.LibraryPath == "" {
		slog.Error("root directory not set in settings")
		return
	}

	c.libraryDirectory = settings.LibraryPath
	c.thumbnailSize = uint(settings.ThumbnailSize)

	/*
	 * For each client, retrieve all their albums. We will use this information
	 * to get a list of images and create cache entries for each album.
	 */
	slog.Info("creating cache for library...")

	pool := pond.NewPool(c.maxCacheWorkers)

	filepath.WalkDir(c.libraryDirectory, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		fileName := d.Name()
		basePath := strings.TrimSuffix(path, fileName)
		ext := strings.ToLower(filepath.Ext(fileName))

		if err = c.ensureCacheDirectoriesExist(basePath); err != nil {
			slog.Error("error ensuring cache directory exists", "error", err)
			return err
		}

		if !validExt[ext] {
			return nil
		}

		pool.Submit(func() {
			if c.doesThumbnailExist(basePath, fileName) {
				return
			}

			slog.Info("creating cached image thumbnail", "path", basePath, "fileName", fileName)
			if err = c.createThumbnail(basePath, fileName); err != nil {
				slog.Error("error creating cached thumbnail", "path", basePath, "fileName", fileName, "error", err)
			}
		})

		return nil
	})

	_ = pool.Stop().Wait()
}

func (c CacheCreatorService) ensureCacheDirectoriesExist(path string) error {
	var (
		err error
	)

	cacheDir := services.GetThumbnailCacheDir(c.libraryDirectory, c.cacheDirectory, path)

	if _, err = os.Stat(cacheDir); errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(cacheDir, 0755); err != nil {
			return fmt.Errorf("error creating thumbnail cache directory path %s: %w", cacheDir, err)
		}
	}

	return nil
}

func (c CacheCreatorService) doesThumbnailExist(path, fileName string) bool {
	var (
		err error
	)

	cacheDir := services.GetThumbnailCacheDir(c.libraryDirectory, c.cacheDirectory, path)
	cachedImagePath := filepath.Join(cacheDir, fileName)

	// Check if the file exists
	_, err = os.Stat(cachedImagePath)
	return err == nil
}

func (c CacheCreatorService) createThumbnail(path, fileName string) error {
	var (
		err error
		f   *os.File
		out *os.File
		img image.Image
	)

	// Construct paths
	cacheDir := services.GetThumbnailCacheDir(c.libraryDirectory, c.cacheDirectory, path)
	sourcePath := filepath.Join(path, fileName)
	cachePath := filepath.Join(cacheDir, fileName)

	if f, err = os.Open(sourcePath); err != nil {
		return fmt.Errorf("error opening source image %s: %w", sourcePath, err)
	}

	defer f.Close()

	if img, _, err = image.Decode(f); err != nil {
		return fmt.Errorf("error decoding image %s: %w", sourcePath, err)
	}

	/*
	 * Create the output file and save the resized image
	 */
	if out, err = os.Create(cachePath); err != nil {
		return fmt.Errorf("error creating cache file %s: %w", cachePath, err)
	}

	defer out.Close()

	resizedImage := c.resize(img, c.thumbnailSize)
	ext := strings.ToLower(filepath.Ext(fileName))

	switch ext {
	case ".jpg", ".jpeg":
		if err = jpeg.Encode(out, resizedImage, &jpeg.Options{Quality: 85}); err != nil {
			return fmt.Errorf("error encoding JPEG image %s: %w", cachePath, err)
		}
	default:
		return fmt.Errorf("unsupported image format: %s", ext)
	}

	return nil
}

func (c CacheCreatorService) resize(img image.Image, maxSize uint) image.Image {
	var (
		resizedImage        image.Image
		newWidth, newHeight uint
	)

	/*
	 * Determine which dimension to resize based on the longest edge
	 */
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())

	if width > height {
		// Landscape orientation
		newWidth = maxSize
		newHeight = uint(float64(height) * (float64(maxSize) / float64(width)))
	} else {
		// Portrait orientation or square
		newHeight = maxSize
		newWidth = uint(float64(width) * (float64(maxSize) / float64(height)))
	}

	resizedImage = resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
	return resizedImage
}
