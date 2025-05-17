package services

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adampresley/ownmyphotos/pkg/models"
)

/*
Returns the relative path from the library path to the photo.
*/
func GetPhotoRelativePath(libraryPath string, photo *models.Photo) string {
	trimmed := strings.TrimPrefix(photo.FullPath, libraryPath)
	trimmed = strings.TrimPrefix(trimmed, string(os.PathSeparator))
	return trimmed
}

func GetFolderRelativePath(libraryPath, folderPath string) string {
	trimmed := strings.TrimPrefix(folderPath, libraryPath)
	trimmed = strings.TrimPrefix(trimmed, string(os.PathSeparator))
	return trimmed
}

func GetRelativePathFromFullPath(libraryPath, fullPath string) string {
	trimmed := strings.TrimPrefix(fullPath, libraryPath)
	trimmed = strings.TrimPrefix(trimmed, string(os.PathSeparator))
	return trimmed
}

/*
GetThumbnailCacheDir returns the full path to the thumbnail cache directory for a given album.
An album path comes from the directory path in a given photo record.
*/
func GetThumbnailCacheDir(libraryPath, cachePath, albumPath string) string {
	pathMinusLibraryRoot := strings.TrimPrefix(albumPath, libraryPath)
	fullPath := filepath.Join(cachePath, pathMinusLibraryRoot, "thumbnails")
	return fullPath
}

/*
GetThumbnailCachePath returns the full path to the thumbnail cache file for a given album and file.
*/
func GetThumbnailCachePath(libraryPath, cachePath, albumPath, fileName, ext string) string {
	return filepath.Join(GetThumbnailCacheDir(libraryPath, cachePath, albumPath), fileName+ext)
}

/*
GetPhotoPath returns the full path to the original photo file for a given album.
*/
func GetPhotoPath(libraryPath, albumPath, fileName, ext string) string {
	pathMinusLibraryRoot := strings.TrimPrefix(albumPath, libraryPath)
	fullPath := filepath.Join(libraryPath, pathMinusLibraryRoot, fileName+ext)
	return fullPath

}
