package home

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/configuration"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/viewmodels"
	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
)

type HomeHandlers interface {
	HomePage(w http.ResponseWriter, r *http.Request)
	AboutPage(w http.ResponseWriter, r *http.Request)
	SimpleSearchPage(w http.ResponseWriter, r *http.Request)
}

type HomeControllerConfig struct {
	Config          *configuration.Config
	FolderService   services.FolderServicer
	PhotoService    services.PhotoServicer
	Renderer        rendering.TemplateRenderer
	SettingsService services.SettingsServicer
}

type HomeController struct {
	config          *configuration.Config
	folderService   services.FolderServicer
	photoService    services.PhotoServicer
	renderer        rendering.TemplateRenderer
	settingsService services.SettingsServicer
}

func NewHomeController(config HomeControllerConfig) HomeController {
	return HomeController{
		config:          config.Config,
		folderService:   config.FolderService,
		photoService:    config.PhotoService,
		renderer:        config.Renderer,
		settingsService: config.SettingsService,
	}
}

func (c HomeController) HomePage(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		settings  *models.Settings
		cleanRoot string
		photos    []*models.Photo
		folders   []*models.Folder
	)

	/*
	 * Ignore metadata queries, like ".well_know"
	 */
	if strings.HasPrefix(r.URL.String(), "/.") {
		http.StatusText(http.StatusNoContent)
		return
	}

	pageName := "pages/home"

	viewData := viewmodels.Home{
		BaseViewModel: viewmodels.BaseViewModel{
			Message: "",
			IsHtmx:  httphelpers.IsHtmx(r),
			JavascriptIncludes: []rendering.JavascriptInclude{
				{Src: "/static/js/fslightbox.js", Type: "text/javascript"},
				{Src: "/static/js/pages/home.js", Type: "module"},
			},
		},
		Root:   strings.TrimSpace(httphelpers.GetFromRequest[string](r, "root")),
		Images: []viewmodels.ImageModel{},
		Parent: "root",
	}

	if settings, err = c.settingsService.Read(); err != nil {
		slog.Error("error reading settings", "error", err)
		viewData.Message = "Error reading settings"
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	if cleanRoot, err = c.config.SanitizePath(settings.LibraryPath, viewData.Root); err != nil {
		slog.Error("error determining root path", "error", err, "root", viewData.Root)
		viewData.Message = "Invalid root path"
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	if viewData.Root != "" {
		pathParts := strings.Split(filepath.ToSlash(viewData.Root), "/")

		if len(pathParts) > 1 {
			viewData.Parent = strings.Join(pathParts[:len(pathParts)-1], "/")
		} else if len(pathParts) == 1 {
			viewData.Parent = ""
		}
	}

	if folders, err = c.folderService.All(); err != nil {
		slog.Error("error getting folders", "error", err, "root", cleanRoot)
		viewData.Message = "There was an error retrieving folders"
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	viewData.Folders = BuildFolderTree(settings.LibraryPath, folders, cleanRoot)
	fmt.Printf("\n\nFOLDERS:\n%s\n\n", viewData.Folders.String())
	/*
	 * Get photos for this path.
	 */
	if photos, err = c.photoService.GetPhotosInFolder(cleanRoot); err != nil {
		slog.Error("error getting photos", "error", err, "root", cleanRoot)
		viewData.Message = "There was an error retrieving photos for the path '" + cleanRoot + "'."
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	if folders, err = c.folderService.GetChildren(services.GetRelativePathFromFullPath(settings.LibraryPath, cleanRoot)); err != nil {
		slog.Error("error getting folders", "error", err, "root", cleanRoot)
		viewData.Message = "There was an error folder information for '" + cleanRoot + "'."
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	viewData.Images = viewmodels.NewImageModelCollectionFromPhotos(photos, folders, settings.LibraryPath)
	c.renderer.Render(pageName, viewData, w)
}

func (c HomeController) SimpleSearchPage(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		settings *models.Settings
	)

	pageName := "pages/simple-search"

	viewData := viewmodels.SimpleSearch{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx:             httphelpers.IsHtmx(r),
			JavascriptIncludes: []rendering.JavascriptInclude{},
		},
		SearchTerm:  httphelpers.GetFromRequest[string](r, "term"),
		Root:        httphelpers.GetFromRequest[string](r, "root"),
		Results:     models.SearchPhotosResult{},
		LibraryPath: "",
	}

	criteria := models.PhotoSearch{
		SearchTerm: viewData.SearchTerm,
	}

	if settings, err = c.settingsService.Read(); err != nil {
		slog.Error("error reading settings", "error", err)
		viewData.Message = "Error reading settings"
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	viewData.LibraryPath = settings.LibraryPath

	if viewData.Results, err = c.photoService.Search(criteria); err != nil {
		slog.Error("error searching for photos", "error", err, "term", viewData.SearchTerm)
		viewData.Message = "There was an error searching for photos with the term '" + viewData.SearchTerm + "'."
		viewData.IsError = true

		c.renderer.Render(pageName, viewData, w)
		return
	}

	c.renderer.Render(pageName, viewData, w)
}

func (c HomeController) AboutPage(w http.ResponseWriter, r *http.Request) {
	pageName := "pages/about"

	viewData := viewmodels.AboutPage{
		BaseViewModel: viewmodels.BaseViewModel{
			Message:            "",
			IsHtmx:             httphelpers.IsHtmx(r),
			JavascriptIncludes: []rendering.JavascriptInclude{},
		},
	}

	c.renderer.Render(pageName, viewData, w)
}

// BuildFolderTree builds a hierarchical folder structure
func BuildFolderTree(libraryPath string, folders []*models.Folder, currentPath string) *models.FolderNode {
	// Create a map of paths to folder nodes
	folderMap := make(map[string]*models.FolderNode)

	// Create root node
	root := &models.FolderNode{
		FullPath:   "",
		FolderName: "Photos",
		ParentPath: "",
		IsOpen:     currentPath == "",
		Children:   []*models.FolderNode{},
	}

	folderMap[""] = root

	// First pass: create all folder nodes
	for _, folder := range folders {
		p := strings.TrimPrefix(strings.TrimPrefix(folder.FullPath, libraryPath), string(os.PathSeparator))
		pp := filepath.Join(libraryPath, folder.ParentPath)

		node := &models.FolderNode{
			FullPath:   folder.FullPath,
			Path:       p,
			FolderName: folder.FolderName,
			ParentPath: pp,
			IsOpen:     strings.HasPrefix(currentPath, folder.FullPath),
			Children:   []*models.FolderNode{},
		}

		folderMap[folder.FullPath] = node
	}

	// Second pass: build the tree structure
	for _, folder := range folders {
		node := folderMap[folder.FullPath]
		parentPath := filepath.Join(libraryPath, folder.ParentPath)

		if folder.ParentPath == "" {
			parentPath = ""
		}

		parent, exists := folderMap[parentPath]

		if exists {
			parent.Children = append(parent.Children, node)
		} else {
			// If parent doesn't exist yet, add to root
			root.Children = append(root.Children, node)
		}
	}

	return root
}
