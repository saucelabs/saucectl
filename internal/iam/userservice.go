package iam

import "context"

// User represents user data structure
type User struct {
	ID           string       `json:"id"`
	Groups       []Group      `json:"groups"`
	Organization Organization `json:"organization"`
}

type Concurrency struct {
	Organization OrgConcurrency `json:"organization"`
}

// OrgConcurrency represents the concurrency for an organization.
type OrgConcurrency struct {
	Allowed CloudConcurrency `json:"allowed"`
}

// CloudConcurrency represents a concurrency per cloud environment.
type CloudConcurrency struct {
	VMS int `json:"vms"`
	RDS int `json:"rds"`
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
	GetUser(context.Context) (User, error)
	// GetConcurrency returns the concurrency settings for the current account.
	GetConcurrency(context.Context) (Concurrency, error)
}
