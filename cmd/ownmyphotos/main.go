package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adampresley/adamgokit/cron"
	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/adamgokit/mux"
	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/configuration"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/home"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/library"
	"github.com/adampresley/ownmyphotos/cmd/ownmyphotos/internal/settings"
	"github.com/adampresley/ownmyphotos/pkg/cache"
	"github.com/adampresley/ownmyphotos/pkg/collector"
	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
	_ "github.com/glebarez/sqlite"
	"github.com/rfberaldo/sqlz"
	"github.com/rfberaldo/sqlz/binds"
)

var (
	Version string = "development"
	appName string = "ownmyphotos"

	//go:embed app
	appFS embed.FS

	config configuration.Config

	/* Services */
	db               *sqlz.DB
	folderService    services.FolderServicer
	jpegCollector    collector.Collector
	jpegCacheCreator cache.CacheCreator
	photoCache       services.PhotoCacher
	photoService     services.PhotoServicer
	renderer         rendering.TemplateRenderer
	settingsService  services.SettingsServicer

	/* Controllers */
	homeController     home.HomeHandlers
	libraryController  library.LibraryHandlers
	settingsController settings.SettingsHandlers
)

func main() {
	var (
		err          error
		userSettings *models.Settings
	)

	config = configuration.LoadConfig()
	setupLogger(&config, Version)

	slog.Info("configuration loaded",
		slog.String("app", appName),
		slog.String("version", Version),
		slog.String("loglevel", config.LogLevel),
		slog.String("host", config.Host),
	)

	slog.Debug("setting up...")

	binds.Register("sqlite", binds.BindByDriver("sqlite3"))
	if db, err = sqlz.Connect("sqlite", config.DSN); err != nil {
		panic(err)
	}

	migrateDatabase()

	/*
	 * Setup services
	 */
	renderer = rendering.NewGoTemplateRenderer(rendering.GoTemplateRendererConfig{
		TemplateDir:       "app",
		TemplateExtension: ".html",
		TemplateFS:        appFS,
		LayoutsDir:        "layouts",
		ComponentsDir:     "components",
	})

	settingsService = services.NewSettingsService(services.SettingsServiceConfig{
		DB: db,
	})

	if userSettings, err = settingsService.Read(); err != nil {
		slog.Error("error getting cache schedule from the database", "error", err)
		return
	}

	photoCache = services.NewPhotoCache(services.PhotoCacheConfig{
		CachePath: config.CacheDirectory,
	})

	photoService = services.NewPhotoService(services.PhotoServiceConfig{
		DB: db,
	})

	folderService = services.NewFolderService(services.FolderServiceConfig{
		DB: db,
	})

	jpegCacheCreator = cache.NewJpegCacheCreator(uint(userSettings.ThumbnailSize))

	jpegCollector, err = collector.NewJpegCollector(collector.JpegCollectorConfig{
		CachePath:     config.CacheDirectory,
		CacheCreator:  jpegCacheCreator,
		FolderService: folderService,
		PhotoCache:    photoCache,
		PhotoService:  photoService,
	})

	if err != nil {
		slog.Error("error setting up the JPEG collector. the cache path is probably incorrect.", "error", err.Error())
		os.Exit(1)
	}

	/*
	 * Setup controllers
	 */
	homeController = home.NewHomeController(home.HomeControllerConfig{
		Config:          &config,
		FolderService:   folderService,
		PhotoService:    photoService,
		Renderer:        renderer,
		SettingsService: settingsService,
	})

	libraryController = library.NewLibraryController(library.LibraryControllerConfig{
		PhotoCache:      photoCache,
		PhotoService:    photoService,
		SettingsService: settingsService,
	})

	settingsController = settings.NewSettingsController(settings.SettingsControllerConfig{
		Config:          &config,
		Renderer:        renderer,
		SettingsService: settingsService,
	})

	/*
	 * Setup router and http server
	 */
	slog.Debug("setting up routes...")

	routes := []mux.Route{
		{Path: "GET /heartbeat", HandlerFunc: heartbeat},
		{Path: "GET /", HandlerFunc: homeController.HomePage},
		{Path: "GET /about", HandlerFunc: homeController.AboutPage},
		{Path: "GET /settings", HandlerFunc: settingsController.SettingsPage},
		{Path: "POST /settings", HandlerFunc: settingsController.SettingsAction},
		{Path: "GET /library/{id}", HandlerFunc: libraryController.ServeImage},
		{Path: "GET /library/{id}/thumbnail", HandlerFunc: libraryController.ServeThumbnail},
	}

	routerConfig := mux.RouterConfig{
		Address:              config.Host,
		Debug:                Version == "development",
		ServeStaticContent:   true,
		StaticContentRootDir: "app",
		StaticContentPrefix:  "/static/",
		StaticFS:             appFS,
	}

	m := mux.SetupRouter(routerConfig, routes)
	httpServer, quit := mux.SetupServer(routerConfig, m)

	/*
	 * Setup cache creator
	 */
	// setupCacheCreator()
	setupCollectors(userSettings)

	/*
	 * Start cron jobs
	 */
	cron.Start()

	/*
	 * Wait for graceful shutdown
	 */
	slog.Info("server started")

	<-quit
	_ = cron.Stop()
	mux.Shutdown(httpServer)
	slog.Info("server stopped")
}

func heartbeat(w http.ResponseWriter, r *http.Request) {
	httphelpers.TextOK(w, "OK")
}

func migrateDatabase() {
	var (
		err  error
		dirs []os.DirEntry
		b    []byte
	)

	if dirs, err = os.ReadDir(config.DataMigrationDir); err != nil {
		panic(err)
	}

	for _, d := range dirs {
		if d.IsDir() {
			continue
		}

		if strings.HasPrefix(d.Name(), "commit") {
			if b, err = os.ReadFile(filepath.Join(config.DataMigrationDir, d.Name())); err != nil {
				panic(err)
			}

			if err = runSqlScript(b); err != nil {
				if !isIgnorableError(err) {
					panic(err)
				}
			}
		}
	}
}

func runSqlScript(b []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	_, err := db.Exec(ctx, string(b))
	return err
}

func isIgnorableError(err error) bool {
	if strings.Contains(err.Error(), "duplicate column") {
		return true
	}

	return false
}

func setupCollectors(settings *models.Settings) {
	var (
		err error
	)

	collectors := []collector.Collector{
		jpegCollector,
	}

	cron.Add(settings.CollectorSchedule, func() {
		errs := []error{}
		es := []error{}

		for _, c := range collectors {
			if es, err = c.Run(settingsService); err != nil {
				slog.Error("error running collector", "error", err)
				return
			}

			errs = append(errs, es...)
		}

		if len(errs) > 0 {
			slog.Error("errors captured during photo collection", "errors", errs)
		}

		slog.Info("photo collection completed")
	})
}
