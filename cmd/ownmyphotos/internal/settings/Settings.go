package settings

import (
	"log/slog"
	"net/http"

	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/configuration"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/viewmodels"
	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
)

type SettingsHandlers interface {
	SettingsPage(w http.ResponseWriter, r *http.Request)
	SettingsAction(w http.ResponseWriter, r *http.Request)
}

type SettingsControllerConfig struct {
	Config          *configuration.Config
	Renderer        rendering.TemplateRenderer
	SettingsService services.SettingsServicer
}

type SettingsController struct {
	config          *configuration.Config
	renderer        rendering.TemplateRenderer
	settingsService services.SettingsServicer
}

func NewSettingsController(config SettingsControllerConfig) SettingsController {
	return SettingsController{
		config:          config.Config,
		renderer:        config.Renderer,
		settingsService: config.SettingsService,
	}
}

/*
GET /settings
*/
func (c SettingsController) SettingsPage(w http.ResponseWriter, r *http.Request) {
	var (
		err error
	)

	pageName := "pages/settings"

	viewData := viewmodels.Settings{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
		},
		Settings: &models.Settings{},
	}

	if viewData.Settings, err = c.settingsService.Read(); err != nil {
		slog.Error("error reading settings", "error", err)
		viewData.IsError = true
		viewData.Message = "Error reading settings. Please review logs for more details."

		c.renderer.Render(pageName, viewData, w)
		return
	}

	c.renderer.Render(pageName, viewData, w)
}

/*
POST /settings
*/
func (c SettingsController) SettingsAction(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		settings models.Settings
	)

	pageName := "pages/settings"

	viewData := viewmodels.Settings{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
		},
		Settings: &models.Settings{},
	}

	settings = models.Settings{
		CollectorSchedule: httphelpers.GetFromRequest[string](r, "collectorSchedule"),
		MaxWorkers:        httphelpers.GetFromRequest[int](r, "maxWorkers"),
		LibraryPath:       httphelpers.GetFromRequest[string](r, "libraryPath"),
		ThumbnailSize:     httphelpers.GetFromRequest[int](r, "thumbnailSize"),
	}

	// Save the settings
	if err = c.settingsService.Save(&settings); err != nil {
		slog.Error("error saving settings", "error", err)
		viewData.IsError = true
		viewData.Message = "Error saving settings. Please review logs for more details."
		viewData.Settings = &settings // Keep the form data

		c.renderer.Render(pageName, viewData, w)
		return
	}

	// If save was successful, read the settings again to ensure we have the latest data
	if viewData.Settings, err = c.settingsService.Read(); err != nil {
		slog.Error("error reading settings after save", "error", err)
		viewData.IsError = true
		viewData.Message = "Settings saved, but there was an error reading them back. Please refresh the page."
	}

	viewData.Message = "Settings saved successfully."
	c.renderer.Render(pageName, viewData, w)
}
