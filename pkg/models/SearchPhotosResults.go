package models

type SearchPhotosResult struct {
	PhotoMatches   []*Photo
	FolderMatches  []*Folder
	KeywordMatches []*KeywordSearchResult
	PeopleMatches  []*Person
}

type KeywordSearchResult struct {
	Keyword    string
	NumMatches int
}
