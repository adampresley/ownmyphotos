package cache

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
)

type JpegCacheCreator struct {
	thumnailSize uint
}

func NewJpegCacheCreator(thumnailSize uint) JpegCacheCreator {
	return JpegCacheCreator{
		thumnailSize: thumnailSize,
	}
}

func (c JpegCacheCreator) DoesExist(cacheFilePath string) bool {
	if _, err := os.Stat(cacheFilePath); err == nil {
		return true
	}

	return false
}

func (c JpegCacheCreator) CreateCacheFile(originalFilePath string, cacheFilePath string) error {
	var (
		err error
		f   *os.File
		out *os.File
		img image.Image
	)

	if f, err = os.Open(originalFilePath); err != nil {
		return fmt.Errorf("error opening source image %s: %w", originalFilePath, err)
	}

	defer f.Close()

	if img, _, err = image.Decode(f); err != nil {
		return fmt.Errorf("error decoding image %s: %w", originalFilePath, err)
	}

	/*
	 * Create the output file and save the resized image
	 */
	if err = os.MkdirAll(filepath.Dir(cacheFilePath), 0755); err != nil {
		return fmt.Errorf("error creating cache directory %s: %w", filepath.Dir(cacheFilePath), err)
	}

	if out, err = os.Create(cacheFilePath); err != nil {
		return fmt.Errorf("error creating cache file %s: %w", cacheFilePath, err)
	}

	defer out.Close()

	resizedImage := c.resize(img, c.thumnailSize)
	ext := strings.ToLower(filepath.Ext(originalFilePath))

	switch ext {
	case ".jpg", ".jpeg":
		if err = jpeg.Encode(out, resizedImage, &jpeg.Options{Quality: 85}); err != nil {
			return fmt.Errorf("error encoding JPEG image %s: %w", cacheFilePath, err)
		}
	default:
		return fmt.Errorf("unsupported image format: %s", ext)
	}

	return nil
}

func (c JpegCacheCreator) resize(img image.Image, maxSize uint) image.Image {
	var (
		resizedImage        image.Image
		newWidth, newHeight uint
	)

	/*
	 * Determine which dimension to resize based on the longest edge
	 */
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())

	if width > height {
		// Landscape orientation
		newWidth = maxSize
		newHeight = uint(float64(height) * (float64(maxSize) / float64(width)))
	} else {
		// Portrait orientation or square
		newHeight = maxSize
		newWidth = uint(float64(width) * (float64(maxSize) / float64(height)))
	}

	resizedImage = resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
	return resizedImage
}
