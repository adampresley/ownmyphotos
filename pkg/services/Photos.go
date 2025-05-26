package services

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/alitto/pond/v2"
	"github.com/rfberaldo/sqlz"
)

type PhotoServicer interface {
	/*
	 * Retrieves all photos from the database. This is mostly
	 * useful for the various image collectors. Other methods
	 * should probably use the paged methods.
	 */
	All() ([]*models.Photo, error)

	/*
	 * Deletes a photo from the database and associated metadata.
	 */
	Delete(id string) error

	/*
	 * Retrieves the OS file ID for a given file path.
	 */
	GetFileID(path string) (string, error)

	/*
	 * Retrieves a single photo by ID.
	 */
	GetPhotoByID(id string) (*models.Photo, error)

	/*
	 * Retrieves all photos in a specific folder.
	 */
	GetPhotosInFolder(folderPath string) ([]*models.Photo, error)

	/*
	 * Saves a photo to the database.
	 */
	Save(photo *models.Photo) error

	/*
	 * Searches for photos based on various criteria. This will
	 * return matches of photos, keywords, and people.
	 */
	Search(criteria models.PhotoSearch) (models.SearchPhotosResult, error)
}

type searchResultSet struct {
	Type    string
	Results []*models.Photo
}

type PhotoServiceConfig struct {
	DB *sqlz.DB
}

type PhotoService struct {
	db *sqlz.DB
}

func NewPhotoService(config PhotoServiceConfig) PhotoService {
	return PhotoService{
		db: config.DB,
	}
}

/*
Retrieves all photos from the database. This is mostly
useful for the various image collectors. Other methods
should probably use the paged methods.
*/
func (s PhotoService) All() ([]*models.Photo, error) {
	var (
		err    error
		result = []*models.Photo{}
	)

	sql := `
SELECT 
	id
	, created_at
	, updated_at
	, deleted_at
	, file_name
	, ext
	, full_path
	, metadata_hash
	, lens_make
	, lens_model
	, lens_id
	, make
	, model
	, caption
	, title
	, creation_date_time
	, width
	, height
	, latitude
	, longitude
	, iptc_digest
	, year
FROM photos 
WHERE 1=1 
	AND deleted_at IS NULL
	`

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.Query(ctx, &result, sql); err != nil {
		return result, fmt.Errorf("error querying for all photos: %w", err)
	}

	for _, photo := range result {
		err = func() error {
			sql := `SELECT keyword FROM photos_keywords WHERE photo_id=?`

			ctx, cancel := DBContext()
			defer cancel()

			if err = s.db.Query(ctx, &photo.Keywords, sql, photo.ID); err != nil {
				return fmt.Errorf("error querying for photo keywords on photo %s: %w", photo.ID, err)
			}

			return nil
		}()

		if err != nil {
			return result, err
		}

		err = func() error {
			people := []*models.Person{}
			sql := `SELECT p.name FROM photos_people AS pp INNER JOIN people AS p ON p.id=pp.person_id WHERE pp.photo_id=?`

			ctx, cancel := DBContext()
			defer cancel()

			if err = s.db.Query(ctx, &people, sql, photo.ID); err != nil {
				return fmt.Errorf("error querying for photo people on photo %s: %w", photo.ID, err)
			}

			photo.People = people
			return nil
		}()

		if err != nil {
			return result, err
		}
	}

	return result, nil
}

/*
Deletes a photo from the database and its associated metadata.
*/
func (s PhotoService) Delete(id string) error {
	var (
		err     error
		success = false
	)

	ctx, cancel := DBContext()
	defer cancel()

	tx, err := s.db.Begin(ctx)

	if err != nil {
		return fmt.Errorf("error starting transaction when deleting photo %s: %w", id, err)
	}

	defer func() {
		if success {
			_ = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}()

	// Delete keyword associations
	sqlStatement := `DELETE FROM photos_keywords WHERE photo_id=?`

	if _, err = tx.Exec(ctx, sqlStatement, id); err != nil {
		return fmt.Errorf("error deleting photo keywords on photo %s: %w", id, err)
	}

	// Delete person associations
	sqlStatement = `DELETE FROM photos_people WHERE photo_id=?`

	if _, err = tx.Exec(ctx, sqlStatement, id); err != nil {
		return fmt.Errorf("error deleting people on photo %s: %w", id, err)
	}

	sqlStatement = `DELETE FROM photos WHERE id=?`

	if _, err = tx.Exec(ctx, sqlStatement, id); err != nil {
		return fmt.Errorf("error deleting photo %s: %w", id, err)
	}

	success = true
	return nil
}

/*
Retrieves the OS file ID for a given file path.
*/
func (s PhotoService) GetFileID(path string) (string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("failed to get inode information")
	}

	result := strconv.FormatUint(stat.Ino, 10)
	return result, nil
}

/*
Retrieves a single photo by ID.
*/
func (s PhotoService) GetPhotoByID(id string) (*models.Photo, error) {
	var (
		err    error
		result = []*models.Photo{}
	)

	/*
	 * Query to get all photos in a specific folder with their metadata
	 * Using JSON aggregation to get keywords and people in a single query
	 */
	sqlStatement := `
SELECT 
    p.id,
    p.created_at,
    p.updated_at,
    p.deleted_at,
    p.file_name,
    p.ext,
    p.full_path,
    p.metadata_hash,
    p.lens_make,
    p.lens_model,
    p.lens_id,
    p.make,
    p.model,
    p.caption,
    p.title,
    p.creation_date_time,
    p.width,
    p.height,
    p.latitude,
    p.longitude,
    p.iptc_digest,
    p.year,
    (
        SELECT json_group_array(pk.keyword)
        FROM photos_keywords pk
        WHERE pk.photo_id = p.id
    ) AS keywords,
    (
        SELECT json_group_array(json_object('id', pe.id, 'name', pe.name))
        FROM photos_people pp
        JOIN people pe ON pp.person_id = pe.id
        WHERE pp.photo_id = p.id
    ) AS people
FROM photos p
WHERE 1=1
	AND p.deleted_at IS NULL
	AND p.id = ?
`

	ctx, cancel := DBContext()
	defer cancel()

	/*
	 * Execute the query
	 */
	if err = s.db.Query(ctx, &result, sqlStatement, id); err != nil {
		if sqlz.IsNotFound(err) {
			return &models.Photo{}, nil
		}

		return &models.Photo{}, fmt.Errorf("error querying for photo %s: %w", id, err)
	}

	if len(result) == 0 {
		return &models.Photo{}, nil
	}

	return result[0], nil
}

/*
Retrieves all photos in a specific folder.
*/
func (s PhotoService) GetPhotosInFolder(folderPath string) ([]*models.Photo, error) {
	var (
		err    error
		result = []*models.Photo{}
	)

	/*
	 * Query to get all photos in a specific folder with their metadata
	 * Using JSON aggregation to get keywords and people in a single query
	 */
	sqlStatement := `
SELECT 
    p.id,
    p.created_at,
    p.updated_at,
    p.deleted_at,
    p.file_name,
    p.ext,
    p.full_path,
    p.metadata_hash,
    p.lens_make,
    p.lens_model,
    p.lens_id,
    p.make,
    p.model,
    p.caption,
    p.title,
    p.creation_date_time,
    p.width,
    p.height,
    p.latitude,
    p.longitude,
    p.iptc_digest,
    p.year,
    (
        SELECT json_group_array(pk.keyword)
        FROM photos_keywords pk
        WHERE pk.photo_id = p.id
    ) AS keywords,
    (
        SELECT json_group_array(json_object('id', pe.id, 'name', pe.name))
        FROM photos_people pp
        JOIN people pe ON pp.person_id = pe.id
        WHERE pp.photo_id = p.id
    ) AS people
FROM photos p
WHERE p.deleted_at IS NULL
AND p.full_path = ?
ORDER BY p.file_name ASC
`

	ctx, cancel := DBContext()
	defer cancel()

	/*
	 * Execute the query
	 */
	if err = s.db.Query(ctx, &result, sqlStatement, folderPath); err != nil {
		return result, fmt.Errorf("error querying for photos in folder %s: %w", folderPath, err)
	}

	return result, nil
}

/*
Saves a photo to the database.
*/
func (s PhotoService) Save(photo *models.Photo) error {
	var (
		err error
		tx  *sqlz.Tx
		r   sql.Result
	)

	statement := ""

	ctx, cancel := DBContext()
	defer cancel()

	if tx, err = s.db.Begin(ctx); err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	for _, keyword := range photo.Keywords {
		statement = `
			INSERT INTO keywords (
				keyword
			) VALUES (
				?
			) ON CONFLICT (keyword) DO NOTHING;
		`

		args := []any{keyword.Keyword, keyword.Keyword}

		if _, err = tx.Exec(ctx, statement, args...); err != nil {
			err2 := tx.Rollback()
			return fmt.Errorf("error inserting keyword %s: %w (%s)", keyword.Keyword, err, err2.Error())
		}
	}

	for _, person := range photo.People {
		// Does this person already exist?
		var personID int64
		statement = `SELECT id FROM people WHERE name=?`

		args := []any{person.Name}

		if err = tx.QueryRow(ctx, &personID, statement, args); err != nil && !sqlz.IsNotFound(err) {
			err2 := tx.Rollback()
			return fmt.Errorf("error querying for person %s: %w (%s)", person.Name, err, err2.Error())
		}

		if sqlz.IsNotFound(err) {
			statement = `
				INSERT INTO people (
					created_at
					, updated_at
					, name
				) VALUES (
					?
					, ?
					, ?
				) ON CONFLICT (name) DO NOTHING;
			`

			args := []any{time.Now().UTC(), time.Now().UTC(), person.Name, time.Now().UTC()}

			if r, err = tx.Exec(ctx, statement, args...); err != nil {
				err2 := tx.Rollback()
				return fmt.Errorf("error inserting person %s: %w (%s)", person.Name, err, err2.Error())
			}

			personID, _ = r.LastInsertId()
		}

		person.ID = uint(personID)
	}

	statement = `
		INSERT INTO photos (
			id
			, created_at
			, updated_at
			, file_name
			, ext
			, full_path
			, metadata_hash
			, lens_make
			, lens_model
			, lens_id
			, make
			, model
			, caption
			, title
			, creation_date_time
			, width
			, height
			, latitude
			, longitude
			, iptc_digest
			, year
		) VALUES (
			?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
			, ?
		) ON CONFLICT (id) DO UPDATE SET
			updated_at=excluded.updated_at
			, file_name=excluded.file_name
			, ext=excluded.ext
			, full_path=excluded.full_path
			, metadata_hash=excluded.metadata_hash
			, lens_make=excluded.lens_make
			, lens_model=excluded.lens_model
			, lens_id=excluded.lens_id
			, make=excluded.make
			, model=excluded.model
			, caption=excluded.caption
			, title=excluded.title
			, creation_date_time=excluded.creation_date_time
			, width=excluded.width
			, height=excluded.height
			, latitude=excluded.latitude
			, longitude=excluded.longitude
			, iptc_digest=excluded.iptc_digest
			, year=excluded.year
	`

	args := []any{
		photo.ID,
		photo.CreatedAt,
		photo.UpdatedAt,
		photo.FileName,
		photo.Ext,
		photo.FullPath,
		photo.MetadataHash,
		photo.LensMake,
		photo.LensModel,
		photo.LensID,
		photo.Make,
		photo.Model,
		photo.Caption,
		photo.Title,
		photo.CreationDateTime,
		photo.Width,
		photo.Height,
		photo.Latitude,
		photo.Longitude,
		photo.IptcDigest,
		photo.Year,
	}

	if _, err = tx.Exec(ctx, statement, args...); err != nil {
		err2 := tx.Rollback()
		return fmt.Errorf("error inserting photo: %w (%s)", err, err2.Error())
	}

	/*
	 * Update associations. First delete associated keywords and people,
	 * then insert new ones.
	 */
	statement = `DELETE FROM photos_people WHERE photo_id=?;`

	if _, err = tx.Exec(ctx, statement, photo.ID); err != nil {
		err2 := tx.Rollback()
		return fmt.Errorf("error deleting people associations from photo: %w (%s)", err, err2.Error())
	}

	statement = `DELETE FROM photos_keywords WHERE photo_id=?;`

	if _, err = tx.Exec(ctx, statement, photo.ID); err != nil {
		err2 := tx.Rollback()
		return fmt.Errorf("error deleting keyword associations from photo: %w (%s)", err, err2.Error())
	}

	for _, person := range photo.People {
		statement = `INSERT INTO photos_people (photo_id, person_id) VALUES (?,?);`

		if _, err = tx.Exec(ctx, statement, photo.ID, person.ID); err != nil {
			err2 := tx.Rollback()
			return fmt.Errorf("error inserting person associations to photo: %w (%s)", err, err2.Error())
		}
	}

	for _, keyword := range photo.Keywords {
		statement = `INSERT INTO photos_keywords (photo_id, keyword) VALUES (?,?)`

		if _, err = tx.Exec(ctx, statement, photo.ID, keyword.Keyword); err != nil {
			err2 := tx.Rollback()
			return fmt.Errorf("error inserting keyword associations to photo: %w (%s)", err, err2.Error())
		}
	}

	/*
	 * Finally! Commit the transaction.
	 */
	err = tx.Commit()
	return err
}

func (s PhotoService) Search(criteria models.PhotoSearch) (models.SearchPhotosResult, error) {
	var (
		result = models.SearchPhotosResult{
			PhotoMatches:   []*models.Photo{},
			FolderMatches:  []*models.Folder{},
			KeywordMatches: []*models.KeywordSearchResult{},
			PeopleMatches:  []*models.Person{},
		}
	)

	pool := pond.NewResultPool[models.SearchPhotosResult](0)
	group := pool.NewGroup()

	// Search for just photos
	_ = group.Submit(func() models.SearchPhotosResult {
		c := models.PhotoSearch{
			SearchTerm: criteria.SearchTerm,
		}

		results, err := s.search(c)

		if err != nil {
			slog.Error("error searching for photos by term", "term", criteria.SearchTerm, "error", err)
			return models.SearchPhotosResult{}
		}

		return models.SearchPhotosResult{
			PhotoMatches: results,
		}
	})

	// Search for photos by keywords
	_ = group.Submit(func() models.SearchPhotosResult {
		c := models.PhotoSearch{
			SearchTerm: criteria.SearchTerm,
		}

		keywordSearchResults, err := s.searchKeywords(c)

		if err != nil {
			slog.Error("error searching for photos by keywords", "keyword", criteria.SearchTerm, "error", err)
			return models.SearchPhotosResult{}
		}

		return models.SearchPhotosResult{KeywordMatches: keywordSearchResults}
	})

	// Search for photos by people
	_ = group.Submit(func() models.SearchPhotosResult {
		results, err := s.searchPeople(models.PhotoSearch{SearchTerm: criteria.SearchTerm})

		if err != nil {
			slog.Error("error searching for photos by people", "person", criteria.SearchTerm)
			return models.SearchPhotosResult{}
		}

		return models.SearchPhotosResult{PeopleMatches: results}
	})

	// Search for folders
	_ = group.Submit(func() models.SearchPhotosResult {
		c := models.PhotoSearch{
			SearchTerm: criteria.SearchTerm,
		}

		results, err := s.searchFolders(c)

		if err != nil {
			slog.Error("error searching for folders by term", "term", criteria.SearchTerm)
			return models.SearchPhotosResult{}
		}

		return models.SearchPhotosResult{FolderMatches: results}
	})

	poolResults, _ := group.Wait()

	for _, resultSet := range poolResults {
		if resultSet.PhotoMatches != nil && len(resultSet.PhotoMatches) > 0 {
			result.PhotoMatches = append(result.PhotoMatches, resultSet.PhotoMatches...)
		}

		if resultSet.KeywordMatches != nil && len(resultSet.KeywordMatches) > 0 {
			result.KeywordMatches = append(result.KeywordMatches, resultSet.KeywordMatches...)
		}

		if resultSet.PeopleMatches != nil && len(resultSet.PeopleMatches) > 0 {
			result.PeopleMatches = append(result.PeopleMatches, resultSet.PeopleMatches...)
		}

		if resultSet.FolderMatches != nil && len(resultSet.FolderMatches) > 0 {
			result.FolderMatches = append(result.FolderMatches, resultSet.FolderMatches...)
		}
	}

	return result, nil
}

func (s PhotoService) search(criteria models.PhotoSearch) ([]*models.Photo, error) {
	var (
		err    error
		photos = []*models.Photo{}
	)

	maxResults := 100
	parameters := []any{}

	statement := `
SELECT 
    p.id,
    p.created_at,
    p.updated_at,
    p.deleted_at,
    p.file_name,
    p.ext,
    p.full_path,
    p.metadata_hash,
    p.lens_make,
    p.lens_model,
    p.lens_id,
    p.make,
    p.model,
    p.caption,
    p.title,
    p.creation_date_time,
    p.width,
    p.height,
    p.latitude,
    p.longitude,
    p.iptc_digest,
    p.year,
    (
        SELECT json_group_array(k.keyword)
        FROM photos_keywords pk
        JOIN keywords k ON pk.keyword = k.keyword
        WHERE pk.photo_id = p.id
    ) AS keywords,
    (
        SELECT json_group_array(json_object('id', pe.id, 'name', pe.name))
        FROM photos_people pp
        JOIN people pe ON pp.person_id = pe.id
        WHERE pp.photo_id = p.id
    ) AS people
FROM photos p
WHERE 1=1
	AND p.deleted_at IS NULL
	`

	// General
	if len(criteria.SearchTerm) > 0 {
		statement += ` 
AND (
	LOWER(p.file_name) LIKE ? 
	OR LOWER(p.title) LIKE ? 
	OR LOWER(p.caption) LIKE ? 
)
		`

		parameters = append(parameters, fmt.Sprintf("%%%s%%", strings.ToLower(criteria.SearchTerm)))
		parameters = append(parameters, fmt.Sprintf("%%%s%%", strings.ToLower(criteria.SearchTerm)))
		parameters = append(parameters, fmt.Sprintf("%%%s%%", strings.ToLower(criteria.SearchTerm)))
	}

	// Keywords
	if len(criteria.Keywords) > 0 {
		statement += ` 
AND p.id IN (
	SELECT photo_id FROM photos_keywords
	WHERE LOWER(keyword) IN (?)

)
		`

		for _, keyword := range criteria.Keywords {
			parameters = append(parameters, strings.ToLower(keyword))
		}
	}

	statement += fmt.Sprintf(` ORDER BY p.creation_date_time LIMIT %d`, maxResults)

	if len(criteria.People) > 0 {
		fmt.Printf("\n\n%s\n\n", statement)
	}

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.Query(ctx, &photos, statement, parameters...); err != nil {
		return photos, fmt.Errorf("error executing search query: %w", err)
	}

	return photos, nil
}

func (s PhotoService) searchPeople(criteria models.PhotoSearch) ([]*models.Person, error) {
	var (
		err     error
		results = []*models.Person{}
	)

	statement := `SELECT id, name FROM people WHERE 1=1 AND LOWER(name) like ?;`

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.Query(ctx, &results, statement, fmt.Sprintf("%%%s%%", strings.ToLower(criteria.SearchTerm))); err != nil {
		return results, fmt.Errorf("error executing search query for people: %w", err)
	}

	return results, nil
}

func (s PhotoService) searchKeywords(criteria models.PhotoSearch) ([]*models.KeywordSearchResult, error) {
	var (
		err     error
		results = []*models.KeywordSearchResult{}
	)

	statement := `
        SELECT 
            k.keyword,
            COUNT(pk.photo_id) as num_matches
        FROM 
            keywords k
        LEFT JOIN 
            photos_keywords pk ON k.keyword = pk.keyword
        WHERE 
            LOWER(k.keyword) LIKE ?
        GROUP BY 
            k.keyword
    `

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.Query(ctx, &results, statement, fmt.Sprintf("%%%s%%", strings.ToLower(criteria.SearchTerm))); err != nil {
		return results, fmt.Errorf("error executing search query for keywords: %w", err)
	}

	return results, nil
}

func (s PhotoService) searchFolders(criteria models.PhotoSearch) ([]*models.Folder, error) {
	var (
		err     error
		results = []*models.Folder{}
	)

	statement := `
        SELECT 
            folder_name,
            parent_path,
            key_photo_id,
            full_path
        FROM 
            folders
        WHERE 
            LOWER(folder_name) = ?
        ORDER BY 
            folder_name ASC
    `

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.Query(ctx, &results, statement, strings.ToLower(criteria.SearchTerm)); err != nil {
		return results, fmt.Errorf("error executing search query for folders: %w", err)
	}

	return results, nil
}
