package collector

import (
	"fmt"

	"github.com/adampresley/ownmyphotos/pkg/services"
)

var (
	ErrCollectorAlreadyRunning = fmt.Errorf("collector is already running")
	ErrInvalidLibraryPath      = fmt.Errorf("invalid library path")
)

type Collector interface {
	Run(settingsService services.SettingsServicer) ([]error, error)
}
