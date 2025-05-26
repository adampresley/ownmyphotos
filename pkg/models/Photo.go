package models

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adampresley/adamgokit/slices"
	"github.com/adampresley/imagemetadata/imagemodel"
)

type Photo struct {
	BaseModel
	ID         string
	mutex      *sync.Mutex `hash:"ignore"`
	TotalCount int         `hash:"ignore"`

	FileName         string
	Ext              string
	FullPath         string
	MetadataHash     string
	LensMake         string
	LensModel        string
	LensID           string
	Make             string
	Model            string
	Keywords         DbKeywordSlice
	Caption          string
	Title            string
	People           DbPeopleSlice
	CreationDateTime time.Time
	Width            int
	Height           int
	Latitude         float64
	Longitude        float64
	IptcDigest       string
	Year             string
}

func NewPhotoFromImageData(imagePathAndName string, imageData *imagemodel.ImageData) *Photo {
	ext := filepath.Ext(imagePathAndName)
	cleanFileName := strings.TrimSuffix(filepath.Base(imagePathAndName), ext)
	creationDateTime := determineCreationDateTime(imageData.CreationDateTime)

	caption := imageData.CaptionEXIF
	title := imageData.TitleXMP

	if caption == "" {
		caption = imageData.CaptionIPTC
	}

	if title == "" {
		title = imageData.TitleIPTC
	}

	result := &Photo{
		mutex: &sync.Mutex{},

		BaseModel: BaseModel{
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		FileName:  cleanFileName,
		Ext:       ext,
		FullPath:  filepath.Dir(imagePathAndName),
		LensMake:  strings.TrimSpace(imageData.LensMake),
		LensModel: strings.TrimSpace(imageData.LensModel),
		LensID:    "",
		Make:      strings.TrimSpace(imageData.Make),
		Model:     strings.TrimSpace(imageData.Model),
		Keywords: slices.Map(imageData.Keywords, func(input string, index int) *Keyword {
			return &Keyword{
				Keyword: input,
			}
		}),
		Caption: strings.TrimSpace(caption),
		Title:   strings.TrimSpace(caption),
		People: slices.Map(imageData.People, func(input string, index int) *Person {
			return &Person{
				Name: input,
			}
		}),
		CreationDateTime: creationDateTime,
		Width:            imageData.Width,
		Height:           imageData.Height,
		Latitude:         imageData.Latitude,
		Longitude:        imageData.Longitude,
		IptcDigest:       "",
		Year:             determineYear(imageData.Keywords, creationDateTime),
	}

	result.MetadataHash = result.GenerateMetadataHash()
	return result
}

func (p *Photo) GenerateMetadataHash() string {
	s := strings.Builder{}
	h := fnv.New64a()

	keywords := slices.Map(p.Keywords, func(k *Keyword, index int) string {
		return k.Keyword
	})

	people := slices.Map(p.People, func(p *Person, index int) string {
		return p.Name
	})

	sort.Strings(keywords)
	sort.Strings(people)

	s.WriteString(p.FullPath + "_")
	s.WriteString(p.FileName + "_")
	s.WriteString(p.LensMake + "_")
	s.WriteString(p.LensModel + "_")
	s.WriteString(p.Make + "_")
	s.WriteString(p.Model + "_")
	s.WriteString(strings.Join(keywords, ",") + "_")
	s.WriteString(strings.Join(people, ",") + "_")
	s.WriteString(p.Caption + "_")
	s.WriteString(p.Title + "_")
	s.WriteString(strconv.FormatFloat(p.Latitude, 'E', -1, 64) + "_")
	s.WriteString(strconv.FormatFloat(p.Longitude, 'E', -1, 64) + "_")
	s.WriteString(strconv.FormatInt(int64(p.Width), 10) + "_")
	s.WriteString(strconv.FormatInt(int64(p.Height), 10) + "_")

	h.Write([]byte(s.String()))
	sum := h.Sum64()

	return strconv.FormatUint(sum, 10)
}

func (p *Photo) String() string {
	r := &strings.Builder{}

	keywords := slices.Map(p.Keywords, func(k *Keyword, index int) string {
		return k.Keyword
	})

	people := slices.Map(p.People, func(p *Person, index int) string {
		return p.Name
	})

	sort.Strings(keywords)
	sort.Strings(people)

	r.WriteString("Photo (" + p.ID + ") '" + p.FileName + "'\n")
	r.WriteString("  Path: " + p.FullPath + "\n")
	r.WriteString("  Title: " + p.Title + "\n")
	r.WriteString("  Caption: " + p.Caption + "\n")
	r.WriteString("  Metadata Hash: " + p.MetadataHash + "\n")
	r.WriteString("  Lens:\n")
	r.WriteString("    Make: " + p.LensMake + "\n")
	r.WriteString("    Model: " + p.LensModel + "\n")
	r.WriteString("    ID:" + p.LensID + "\n")
	r.WriteString("  Camera:\n")
	r.WriteString("    Make:" + p.Make + "\n")
	r.WriteString("    Model:" + p.Model + "\n")
	r.WriteString("  Keywords: " + strings.Join(keywords, " ") + "\n")
	r.WriteString("  People: " + strings.Join(people, ", ") + "\n")
	r.WriteString("  Width: " + strconv.Itoa(p.Width) + "\n")
	r.WriteString("  Height: " + strconv.Itoa(p.Height) + "\n")
	r.WriteString("  Year: " + p.Year + "\n")

	return r.String()
}

/*
Returns the relative path to the photo within the library.
*/
func (p *Photo) GetAlbumPath(libraryPath string) string {
	pathMinusLibrary := strings.TrimPrefix(p.FullPath, libraryPath)
	pathMinusLibrary = strings.TrimPrefix(pathMinusLibrary, string(os.PathSeparator))
	return pathMinusLibrary
}

func (p *Photo) GetFullPath() string {
	return filepath.Join(p.FullPath, p.FileName+p.Ext)
}

func determineCreationDateTime(dateTimeString string) time.Time {
	result, err := time.Parse("2006-01-02T15:04:05", dateTimeString)

	if err != nil {
		return time.Time{}
	}

	return result
}

func determineYear(keywords []string, creationDateTime time.Time) string {
	r := regexp.MustCompile("^[0-9]{4}$")

	for _, keyword := range keywords {
		if r.MatchString(keyword) {
			return keyword
		}
	}

	return creationDateTime.Format("2006")
}
