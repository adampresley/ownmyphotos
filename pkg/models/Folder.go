package models

type Folder struct {
	FolderName string
	ParentPath string
	KeyPhotoID string
	KeyPhoto   *Photo
	FullPath   string
}
