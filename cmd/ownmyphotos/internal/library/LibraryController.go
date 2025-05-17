package library

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
)

type LibraryHandlers interface {
	ServeImage(w http.ResponseWriter, r *http.Request)
	ServeThumbnail(w http.ResponseWriter, r *http.Request)
}

type LibraryControllerConfig struct {
	PhotoCache      services.PhotoCacher
	PhotoService    services.PhotoServicer
	SettingsService services.SettingsServicer
}

type LibraryController struct {
	photoCache      services.PhotoCacher
	photoService    services.PhotoServicer
	settingsService services.SettingsServicer
}

func NewLibraryController(config LibraryControllerConfig) LibraryController {
	return LibraryController{
		photoCache:      config.PhotoCache,
		photoService:    config.PhotoService,
		settingsService: config.SettingsService,
	}
}

func (c LibraryController) ServeImage(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		settings *models.Settings
		photo    *models.Photo
	)

	id := httphelpers.GetFromRequest[string](r, "id")

	if settings, err = c.settingsService.Read(); err != nil {
		slog.Error("Error reading settings in ServeImage", "error", err)
		http.Error(w, "Error reading settings", http.StatusInternalServerError)
		return
	}

	if photo, err = c.photoService.GetPhotoByID(id); err != nil {
		slog.Error("Error retrieving photo", "error", err, "id", id)
		http.Error(w, "Error retrieving photo", http.StatusNotFound)
		return
	}

	c.serveFullImage(w, r, settings, photo)
}

func (c LibraryController) ServeThumbnail(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		settings *models.Settings
		photo    *models.Photo
	)

	id := httphelpers.GetFromRequest[string](r, "id")

	if settings, err = c.settingsService.Read(); err != nil {
		slog.Error("Error reading settings in ServeImage", "error", err)
		http.Error(w, "Error reading settings", http.StatusInternalServerError)
		return
	}

	if photo, err = c.photoService.GetPhotoByID(id); err != nil {
		slog.Error("Error retrieving photo", "error", err, "id", id)
		http.Error(w, "Error retrieving photo", http.StatusNotFound)
		return
	}

	if c.photoCache.Exists(settings, photo) {
		c.serveThumbnail(w, r, settings, photo)
		return
	}

	c.serveFullImage(w, r, settings, photo)
}

func (c LibraryController) serveFullImage(w http.ResponseWriter, r *http.Request, settings *models.Settings, photo *models.Photo) {
	var (
		err  error
		f    *os.File
		info fs.FileInfo
	)

	fullPath := filepath.Join(photo.FullPath, photo.FileName+photo.Ext)

	if f, err = os.Open(fullPath); err != nil {
		slog.Error("Error opening image file", "error", err, "path", fullPath)
		http.Error(w, "Error retrieving image", http.StatusInternalServerError)
		return
	}

	defer f.Close()

	modTime := time.Now()

	if info, err = os.Stat(fullPath); err == nil {
		modTime = info.ModTime()
	}

	http.ServeContent(w, r, fmt.Sprintf("%s%s", photo.FileName, photo.Ext), modTime, f)
}

func (c LibraryController) serveThumbnail(w http.ResponseWriter, r *http.Request, settings *models.Settings, photo *models.Photo) {
	var (
		err  error
		f    *os.File
		info fs.FileInfo
	)

	fullPath := c.photoCache.GetFullCachePath(settings, photo)

	if f, err = os.Open(fullPath); err != nil {
		slog.Error("Error opening cached image file", "error", err, "path", fullPath)
		http.Error(w, "Error retrieving cached image", http.StatusInternalServerError)
		return
	}

	defer f.Close()

	modTime := time.Now()

	if info, err = os.Stat(fullPath); err == nil {
		modTime = info.ModTime()
	}

	http.ServeContent(w, r, fmt.Sprintf("%s%s", photo.FileName, photo.Ext), modTime, f)
}
