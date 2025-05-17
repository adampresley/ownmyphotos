package models

type PhotoSearch struct {
	Keywords   []string
	Page       int
	People     []int
	SearchTerm string
}
