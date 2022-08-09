package iam

import "context"

// User represents user data structure
type User struct {
	ID           string       `json:"id"`
	Groups       []Group      `json:"groups"`
	Organization Organization `json:"organization"`
}

// Group represents the group that the user belongs to
type Group struct {
	ID string `json:"id"`
}

// Organization represents the organization that the user belongs to
type Organization struct {
	ID string `json:"id"`
}

type Service interface {
	Get(context.Context) (User, error)
}
