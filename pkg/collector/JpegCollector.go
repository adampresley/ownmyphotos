package collector

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adampresley/adamgokit/datastructures"
	"github.com/adampresley/adamgokit/slices"
	"github.com/adampresley/imagemetadata"
	"github.com/adampresley/imagemetadata/imagemodel"
	"github.com/adampresley/ownmyphotos/pkg/cache"
	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
	"github.com/alitto/pond/v2"
)

type JpegCollectorConfig struct {
	CachePath     string
	CacheCreator  cache.CacheCreator
	FolderService services.FolderServicer
	PhotoCache    services.PhotoCacher
	PhotoService  services.PhotoServicer
}

type JpegCollector struct {
	cachePath     string
	cacheCreator  cache.CacheCreator
	folderService services.FolderServicer
	photoCache    services.PhotoCacher
	photoService  services.PhotoServicer

	pool    pond.Pool
	running bool
}

func NewJpegCollector(config JpegCollectorConfig) (*JpegCollector, error) {
	var (
		err error
	)

	// Ensure cache path exists
	if _, err = os.Stat(config.CachePath); os.IsNotExist(err) {
		if err = os.MkdirAll(config.CachePath, 0755); err != nil {
			return &JpegCollector{}, fmt.Errorf("error creating cache directory: %w", err)
		}
	}

	return &JpegCollector{
		cachePath:     config.CachePath,
		cacheCreator:  config.CacheCreator,
		folderService: config.FolderService,
		photoCache:    config.PhotoCache,
		photoService:  config.PhotoService,
		running:       false,
	}, nil
}

func (c *JpegCollector) Run(settingsService services.SettingsServicer) ([]error, error) {
	var (
		err           error
		processErrors []error
		allPhotos     []*models.Photo
		settings      *models.Settings
	)

	if c.running {
		return []error{}, ErrCollectorAlreadyRunning
	}

	c.running = true
	defer func() {
		c.running = false
	}()

	if settings, err = settingsService.Read(); err != nil {
		return []error{}, fmt.Errorf("error reading settings: %w", err)
	}

	/*
	 * Verify the library path exists. If not, return an error.
	 */
	if _, err := os.Stat(settings.LibraryPath); os.IsNotExist(err) {
		return []error{}, ErrInvalidLibraryPath
	}

	slog.Info("starting JpegCollector", "maxWorkers", settings.MaxWorkers, "libraryPath", settings.LibraryPath, "cachePath", c.cachePath)

	if allPhotos, err = c.photoService.All(); err != nil {
		return []error{}, fmt.Errorf("error retrieving all photos: %w", err)
	}

	slog.Info("retrieved all database photos", "count", len(allPhotos))

	/*
	 * Clean removed photos from the library and the database.
	 */
	processErrors = append(processErrors, c.cleanRemovedPhotos(settings, allPhotos)...)
	processErrors = append(processErrors, c.syncPhotos(settings, allPhotos)...)

	return processErrors, nil
}

func (c *JpegCollector) cleanRemovedPhotos(settings *models.Settings, allPhotos []*models.Photo) []error {
	var (
		err  error
		errs []error
	)

	for _, photo := range allPhotos {
		fullPath := photo.GetFullPath()

		if _, err = os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
			cacheDir := services.GetThumbnailCacheDir(
				settings.LibraryPath,
				c.cachePath,
				photo.GetAlbumPath(settings.LibraryPath),
			)

			cachePath := services.GetThumbnailCachePath(
				settings.LibraryPath,
				c.cachePath,
				photo.GetAlbumPath(settings.LibraryPath),
				photo.FileName,
				photo.Ext,
			)

			slog.Info("removing photo", "id", photo.ID, "fullPath", fullPath, "cachePath", cachePath)

			if err = c.photoService.Delete(photo.ID); err != nil {
				errs = append(errs, fmt.Errorf("could not delete photo '%s': %w", photo.ID, err))
				continue
			}

			if err = os.Remove(cachePath); err != nil {
				errs = append(errs, fmt.Errorf("could not remove cache file '%s': %w", cachePath, err))
				continue
			}

			// If the thumbnails directory is empty, remove it
			if err = c.cleanEmptyCacheDirectories(settings.LibraryPath, cacheDir); err != nil {
				errs = append(errs, fmt.Errorf("could not clean empty cache directories: %w", err))
			}
		}
	}

	return errs
}

func (c *JpegCollector) syncPhotos(settings *models.Settings, allPhotos []*models.Photo) []error {
	var (
		errs []error
	)

	pool := pond.NewResultPool[[]error](settings.MaxWorkers)
	group := pool.NewGroup()

	dirStack := datastructures.NewStack[*models.Folder]()

	filepath.WalkDir(settings.LibraryPath, func(path string, d os.DirEntry, err error) error {
		var (
			fileID    string
			f         *os.File
			imageData *imagemodel.ImageData
		)

		if err != nil {
			return err
		}

		if d.IsDir() {
			folder := strings.TrimPrefix(path, settings.LibraryPath)
			parentPath := ""

			// get the last folder
			splitPath := strings.Split(folder, string(os.PathSeparator))

			if len(splitPath) > 1 {
				folder = splitPath[len(splitPath)-1]
				parentPath = strings.Join(splitPath[:len(splitPath)-1], string(os.PathSeparator))
			}

			newFolder := &models.Folder{
				FolderName: folder,
				ParentPath: parentPath,
				KeyPhotoID: "",
				FullPath:   path,
			}

			topOfStack := &models.Folder{}

			if dirStack.Size() > 0 {
				topOfStack = dirStack.Top()
			}

			/*
			 * If this is a new folder, and it's not a child of the top folder on the stack,
			 * pop from the stack until we find a folder that is a child of the top folder on the stack.
			 */
			if topOfStack != nil && len(splitPath) < dirStack.Size() {
				for i := len(splitPath) - 1; i > 0; i-- {
					combined := strings.Join(splitPath[:i], string(os.PathSeparator))
					if combined == dirStack.Top().FolderName {
						break
					}

					_ = dirStack.Pop()
				}
			}

			dirStack.Push(newFolder)

			if err = c.folderService.Save(newFolder); err != nil {
				errs = append(errs, fmt.Errorf("could not save folder '%s': %w", path, err))
				return err
			}

			return nil
		}

		/*
		 * Skip non-JPEG files.
		 */
		ext := filepath.Ext(path)
		lowerExt := strings.ToLower(ext)

		if lowerExt != ".jpg" && lowerExt != ".jpeg" {
			return nil
		}

		group.Submit(func() []error {
			errs := []error{}

			albumPath := strings.TrimPrefix(filepath.Dir(strings.TrimPrefix(path, settings.LibraryPath)), string(os.PathSeparator))
			fileName := strings.TrimSuffix(filepath.Base(path), ext)
			fullImagePath := services.GetPhotoPath(settings.LibraryPath, albumPath, fileName, ext)
			fullCachePath := services.GetThumbnailCachePath(settings.LibraryPath, c.cachePath, albumPath, fileName, ext)

			/*
			 * Open the photo and extract metadata.
			 */
			if f, err = os.Open(fullImagePath); err != nil {
				errs = append(errs, fmt.Errorf("could not open file '%s': %w", fullImagePath, err))
				return errs
			}

			defer f.Close()

			if imageData, err = imagemetadata.NewFromJPEG(f); err != nil {
				errs = append(errs, fmt.Errorf("could not extract metadata from file '%s': %w", fullImagePath, err))
				return errs
			}

			/*
			 * Find an existing photo, if any, in the database. This will help
			 * us determine if we need to create a new record, or update an existing one.
			 */
			existingPhoto := slices.Find(allPhotos, func(p *models.Photo) bool {
				fullPath := filepath.Join(settings.LibraryPath, albumPath)
				if p.FileName == fileName && p.Ext == ext && p.FullPath == fullPath {
					return true
				}

				return false
			})

			filePhoto := models.NewPhotoFromImageData(
				fullImagePath,
				imageData,
			)

			if fileID, err = c.photoService.GetFileID(fullImagePath); err != nil {
				errs = append(errs, fmt.Errorf("could not get file ID for '%s': %w", fullImagePath, err))
				return errs
			}

			if existingPhoto == nil {
				existingPhoto = &models.Photo{}
			}

			if existingPhoto.ID != fileID || existingPhoto.MetadataHash != filePhoto.MetadataHash {
				action := "creating"

				if err = c.cacheCreator.CreateCacheFile(fullImagePath, fullCachePath); err != nil {
					errs = append(errs, fmt.Errorf("could not create cache file for '%s': %w", fullImagePath, err))
					return errs
				}

				// Determine what we should do with the photo: update or create
				filePhoto.ID = fileID

				if existingPhoto.ID == fileID {
					filePhoto.CreatedAt = existingPhoto.CreatedAt
					filePhoto.UpdatedAt = time.Now().UTC()
					action = "updating"
				}

				slog.Info(action+" photo", "path", fullImagePath, "fileID", fileID, "metadataHash", filePhoto.MetadataHash, "existingID", existingPhoto.ID, "existingHash", existingPhoto.MetadataHash)

				if err = c.photoService.Save(filePhoto); err != nil {
					slog.Error("error saving photo", "error", err, "filename", fileName, "ext", ext)
					errs = append(errs, fmt.Errorf("could not save photo '%s': %w", fileName, err))
					return errs
				}
			} else if !c.cacheCreator.DoesExist(fullCachePath) {
				slog.Info("creating cache file for photo", "path", fullCachePath)
				if err = c.cacheCreator.CreateCacheFile(fullImagePath, fullCachePath); err != nil {
					errs = append(errs, fmt.Errorf("could not create cache file for '%s': %w", fullImagePath, err))
					return errs
				}
			}

			return []error{}
		})

		return nil
	})

	result, _ := group.Wait()

	for _, groupErrors := range result {
		errs = append(errs, groupErrors...)
	}

	return errs
}

// cleanEmptyCacheDirectories removes empty cache directories recursively
// It starts with the thumbnails directory and works its way up to parent directories
func (c *JpegCollector) cleanEmptyCacheDirectories(libraryPath, thumbnailsDir string) error {
	slog.Info("cleaning empty cache directories", "path", thumbnailsDir)

	// First check if the thumbnails directory exists
	if _, err := os.Stat(thumbnailsDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Check if thumbnails directory is empty
	isEmpty, err := isDirEmpty(thumbnailsDir)
	if err != nil {
		return fmt.Errorf("error checking if directory is empty: %w", err)
	}

	if !isEmpty {
		return nil // Directory is not empty, don't remove it
	}

	// Remove the thumbnails directory
	slog.Info("removing empty thumbnails directory", "path", thumbnailsDir)
	if err := os.Remove(thumbnailsDir); err != nil {
		return fmt.Errorf("error removing thumbnails directory: %w", err)
	}

	// Now check if the parent directory (Event) is empty
	parentDir := filepath.Dir(thumbnailsDir)
	isEmpty, err = isDirEmpty(parentDir)

	if err != nil {
		return fmt.Errorf("error checking if event directory is empty: %w", err)
	}

	if !isEmpty {
		return nil // Event directory is not empty, don't remove it
	}

	// Remove the parent directory
	slog.Info("removing empty parent directory", "path", parentDir)

	fldr := &models.Folder{
		FullPath: filepath.Join(libraryPath, strings.TrimPrefix(parentDir, c.cachePath)),
	}

	if err = c.folderService.Delete(fldr); err != nil {
		return fmt.Errorf("error deleting folder in database: %w", err)
	}

	if err := os.Remove(parentDir); err != nil {
		return fmt.Errorf("error removing event directory: %w", err)
	}

	return nil
}

// isDirEmpty checks if a directory is empty
func isDirEmpty(dirPath string) (bool, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read just one entry
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil // Directory is empty
	}
	return false, err // Either not empty or error
}
