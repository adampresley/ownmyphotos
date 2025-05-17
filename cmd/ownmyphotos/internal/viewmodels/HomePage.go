package viewmodels

import (
	"fmt"
	"html/template"

	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/adampresley/ownmyphotos/pkg/services"
)

type Home struct {
	BaseViewModel
	Images []ImageModel
	Root   string
	Parent string
}

type ImageModel struct {
	Ext          string
	IsDirectory  bool
	DirPath      string
	RelativePath string
	Name         template.HTML
	Photo        *models.Photo
	IsFavorite   bool
}

func NewImageModelCollectionFromPhotos(photos []*models.Photo, childFolders []*models.Folder, libraryPath string) []ImageModel {
	var (
		result = []ImageModel{}
	)

	// First add folders
	for _, folder := range childFolders {
		if folder.FolderName != "" {
			result = append(result, ImageModel{
				IsDirectory:  true,
				DirPath:      folder.FullPath,
				RelativePath: services.GetFolderRelativePath(libraryPath, folder.FullPath),
				Name:         template.HTML(folder.FolderName),
			})
		}
	}

	fmt.Printf("child folders: %+v\n", childFolders)

	// Then add photos
	for _, photo := range photos {
		result = append(result, ImageModel{
			Ext:          photo.Ext,
			IsDirectory:  false,
			DirPath:      photo.FullPath,
			RelativePath: services.GetPhotoRelativePath(libraryPath, photo),
			Name:         template.HTML(photo.FileName),
			Photo:        photo,
		})
	}

	return result
}
