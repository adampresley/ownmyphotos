package viewmodels

import "github.com/adampresley/ownmyphotos/pkg/models"

type SimpleSearch struct {
	BaseViewModel
	SearchTerm  string
	Root        string
	Results     models.SearchPhotosResult
	LibraryPath string
}
