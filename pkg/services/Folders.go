package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/adampresley/ownmyphotos/pkg/models"
	"github.com/rfberaldo/sqlz"
)

type FolderServicer interface {
	Delete(folder *models.Folder) error

	/*
	 * Retrieve all folders under a given parent path. parentPath should be
	 * an a full, absolute path.
	 */
	GetChildren(parentPath string) ([]*models.Folder, error)
	Save(folder *models.Folder) error
}

type FolderServiceConfig struct {
	DB *sqlz.DB
}

type FolderService struct {
	db *sqlz.DB
}

func NewFolderService(config FolderServiceConfig) FolderService {
	return FolderService{
		db: config.DB,
	}
}

func (s FolderService) Delete(folder *models.Folder) error {
	var (
		err error
	)

	sql := `
DELETE FROM folders
WHERE 1=1
	AND folder_name=?
	AND parent_path=?
	`

	ctx, cancel := DBContext()
	defer cancel()

	args := []any{
		folder.FolderName,
		folder.ParentPath,
	}

	if _, err = s.db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("error deleting folder: %w", err)
	}

	return nil
}

/*
Retrieve all folders under a given parent path. parentPath should be
an a full, absolute path.
*/
func (s FolderService) GetChildren(parentPath string) ([]*models.Folder, error) {
	var (
		err    error
		result = []*models.Folder{}
	)

	statement := `
SELECT 
	full_path
	, folder_name
	, parent_path
	, key_photo_id
FROM folders
WHERE 1=1
	AND parent_path=?
ORDER BY folder_name ASC
	`

	if parentPath == "." {
		parentPath = ""
	}

	ctx, cancel := DBContext()
	defer cancel()

	if err = s.db.Query(ctx, &result, statement, parentPath); err != nil {
		return result, fmt.Errorf("error getting folder children of '%s': %w", parentPath, err)
	}

	return result, nil
}

func (s FolderService) Save(folder *models.Folder) error {
	var (
		err error
	)

	statement := ` 
INSERT INTO folders (
	full_path
	, folder_name
	, parent_path
	, key_photo_id
) VALUES (
	?
	, ?
	, ?
	, ?
) ON CONFLICT (full_path) DO
UPDATE SET
	key_photo_id=excluded.key_photo_id
	, folder_name=excluded.folder_name
	, parent_path=excluded.parent_path
	`

	args := []any{
		folder.FullPath,
		folder.FolderName,
		strings.TrimPrefix(folder.ParentPath, string(os.PathSeparator)),
		folder.KeyPhotoID,
	}

	ctx, cancel := DBContext()
	defer cancel()

	if _, err = s.db.Exec(ctx, statement, args...); err != nil {
		return fmt.Errorf("error saving folder: %w", err)
	}

	return nil
}
