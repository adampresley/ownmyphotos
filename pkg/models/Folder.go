package models

import (
	"os"
	"strings"
)

type Folder struct {
	FolderName string
	ParentPath string
	KeyPhotoID string
	KeyPhoto   *Photo
	FullPath   string
}

func (f Folder) RelativePath(libraryPath string) string {
	result := strings.TrimPrefix(f.FullPath, libraryPath)
	result = strings.TrimPrefix(result, string(os.PathSeparator))
	return result
}
