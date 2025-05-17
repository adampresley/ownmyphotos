package viewmodels

import (
	"github.com/adampresley/adamgokit/rendering"
)

type BaseViewModel struct {
	Message            string
	IsError            bool
	IsWarning          bool
	IsHtmx             bool
	JavascriptIncludes []rendering.JavascriptInclude
}
