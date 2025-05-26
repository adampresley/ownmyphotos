package models

import (
	"fmt"
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

// FolderNode represents a node in the folder tree structure
type FolderNode struct {
	FullPath   string
	Path       string
	FolderName string
	ParentPath string
	IsOpen     bool
	Children   []*FolderNode
}

func (f FolderNode) String() string {
	r := strings.Builder{}

	r.WriteString(fmt.Sprintf(`'%s' ('%s'):\n`, f.FolderName, f.Path))

	if f.Children != nil && len(f.Children) > 0 {
		addChildren(&f, &r, 2)
	}

	return r.String()
}

func addChildren(node *FolderNode, r *strings.Builder, indentation int) {
	r.WriteString("STARTING NEW CHILD LOOP\n")
	if node.Children != nil && len(node.Children) > 0 {
		for _, child := range node.Children {
			r.WriteString(fmt.Sprintf("%s--'%s' ('%s' - '%s')\n", strings.Repeat(" ", indentation), child.FolderName, child.Path, child.ParentPath))

			if child.Children != nil && len(child.Children) > 0 {
				addChildren(child, r, indentation+2)
			}
		}
	}
}
