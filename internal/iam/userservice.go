package iam

import "context"

// User represents user data structure
type User struct {
	ID           string       `json:"id"`
	Groups       []Group      `json:"groups"`
	Organization Organization `json:"organization"`
}

// Concurrency represents the concurrency for an account.
type Concurrency struct {
	Org OrgConcurrency `json:"organization"`
}

// OrgConcurrency represents the concurrency for an organization.
type OrgConcurrency struct {
	Allowed CloudConcurrency `json:"allowed"`
}

// CloudConcurrency represents a concurrency per cloud environment.
type CloudConcurrency struct {
	VDC int `json:"vms"`
	RDC int `json:"rds"`
}

// Group represents the group that the user belongs to
type Group struct {
	ID string `json:"id"`
}

// Organization represents the organization that the user belongs to
type Organization struct {
	ID string `json:"id"`
}

type UserService interface {
	User(context.Context) (User, error)
	// Concurrency returns the concurrency settings for the current account.
	Concurrency(context.Context) (Concurrency, error)
}
