package cache

type CacheCreator interface {
	DoesExist(cacheFilePath string) bool
	CreateCacheFile(originalFilePath string, cacheFilePath string) error
}
