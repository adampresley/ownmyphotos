package configuration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/app-nerds/configinator"
)

type Config struct {
	CacheDirectory   string `flag:"ccd" env:"CACHE_DIRECTORY" default:"../../cache" description:"Cache directory"`
	DataMigrationDir string `flag:"dmd" env:"DATA_MIGRATION_DIR" default:"../../sql-migrations" description:"Migration folder"`
	DSN              string `flag:"dsn" env:"DSN" default:"file:./data/ownmyphotos.db" description:"Database connection"`
	Host             string `flag:"host" env:"HOST" default:"localhost:8080" description:"The address and port to bind the HTTP server to"`
	LogLevel         string `flag:"loglevel" env:"LOG_LEVEL" default:"debug" description:"The log level to use. Valid values are 'debug', 'info', 'warn', and 'error'"`
	MaxCacheWorkers  int    `flag:"mcw" env:"MAX_CACHE_WORKERS" default:"5" description:"Number of concurrent cache workers"`
}

func LoadConfig() Config {
	config := Config{}
	configinator.Behold(&config)
	return config
}

// SanitizePath ensures that a given path cannot traverse outside the library folder.
// It returns a safe, absolute path within the library folder, or an empty string if the path
// would escape the library folder boundary.
func (c *Config) SanitizePath(libraryPath, requestedPath string) (string, error) {
	libraryFolderAbs, _ := filepath.Abs(libraryPath)

	// Ensure the upload folder exists
	if _, err := os.Stat(libraryFolderAbs); os.IsNotExist(err) {
		if err := os.MkdirAll(libraryFolderAbs, 0755); err != nil {
			return "", err
		}
	}

	// Join the requested path with the upload folder
	// This handles both absolute and relative paths
	targetPath := filepath.Join(libraryFolderAbs, filepath.Clean(requestedPath))

	// Clean the path to resolve any ".." or "." components
	targetPath = filepath.Clean(targetPath)

	// Ensure the target path is still within the upload folder
	if !strings.HasPrefix(targetPath, libraryFolderAbs) {
		return "", fmt.Errorf("Invalid path traversal attempt: %s", requestedPath)
	}

	return targetPath, nil
}
