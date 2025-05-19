package models

type PhotoSearch struct {
	Keywords   []string
	Page       int
	People     []string
	SearchTerm string
}
