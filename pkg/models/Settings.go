package models

type Settings struct {
	ID                uint
	CollectorSchedule string
	MaxWorkers        int
	LibraryPath       string
	ThumbnailSize     int
}
