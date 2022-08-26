package storage

import "time"

// ListOptions represents parameters that modify the file list behavior.
type ListOptions struct {
	// Q is any search term (such as app name, file name, description, build number or version) by which you want to filter.
	Q string

	// Name The file name (case-insensitive) by which you want to filter.
	Name string
}

type List struct {
	Items     []Item `json:"items"`
	Truncated bool   `json:"truncated"`
}

type Item struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Size     int       `json:"size"`
	Uploaded time.Time `json:"uploaded"`
}
