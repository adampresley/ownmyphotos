package models

import (
	"encoding/json"
	"fmt"
)

type DbStringSlice []string
type DbKeywordSlice []*Keyword
type DbPeopleSlice []*Person

func (s *DbStringSlice) Scan(src any) error {
	var (
		b []byte
	)

	switch v := src.(type) {
	case string:
		b = []byte(v)

	case []byte:
		b = v

	default:
		return fmt.Errorf("can't scan type %T into DbStringSlice", v)
	}

	return json.Unmarshal(b, s)
}

func (s *DbKeywordSlice) Scan(src any) error {
	var (
		b []byte
	)

	switch v := src.(type) {
	case string:
		b = []byte(v)

	case []byte:
		b = v

	default:
		return fmt.Errorf("can't scan type %T into DbStringSlice", v)
	}

	// First try to unmarshal directly into the slice of Keyword structs
	err := json.Unmarshal(b, s)
	if err == nil {
		return nil
	}

	// If that fails, try to unmarshal as array of strings and convert to Keyword objects
	var keywordStrings []string
	if err := json.Unmarshal(b, &keywordStrings); err != nil {
		return fmt.Errorf("failed to unmarshal keyword data: %w", err)
	}

	// Clear the slice and populate with new data
	*s = (*s)[:0]

	for _, k := range keywordStrings {
		*s = append(*s, &Keyword{Keyword: k})
	}

	return nil
}

func (s *DbPeopleSlice) Scan(src any) error {
	var (
		b []byte
	)

	switch v := src.(type) {
	case string:
		b = []byte(v)

	case []byte:
		b = v

	default:
		return fmt.Errorf("can't scan type %T into DbPeopleSlice", v)
	}

	return json.Unmarshal(b, s)
}
