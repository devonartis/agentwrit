package token

import (
	"errors"
	"strings"
)

var (
	ErrScopeEmpty  = errors.New("scope is empty")
	ErrScopeFormat = errors.New("scope format must be action:resource:identifier")
)

type ScopeParts struct {
	Action     string
	Resource   string
	Identifier string
}

func ParseScope(s string) (*ScopeParts, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil, ErrScopeEmpty
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return nil, ErrScopeFormat
	}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return nil, ErrScopeFormat
		}
	}
	return &ScopeParts{
		Action:     parts[0],
		Resource:   parts[1],
		Identifier: parts[2],
	}, nil
}

// ScopeMatch returns true when available scope satisfies required scope.
func ScopeMatch(required, available string) bool {
	req, err := ParseScope(required)
	if err != nil {
		return false
	}
	have, err := ParseScope(available)
	if err != nil {
		return false
	}
	if req.Action != have.Action {
		return false
	}
	if req.Resource != have.Resource {
		return false
	}
	// Wildcard scope grants access to any identifier for the same action/resource.
	if have.Identifier == "*" {
		return true
	}
	return req.Identifier == have.Identifier
}

// ScopeIsSubset ensures every child scope is covered by at least one parent scope.
func ScopeIsSubset(child, parent []string) bool {
	if len(child) == 0 {
		return true
	}
	if len(parent) == 0 {
		return false
	}
	for _, c := range child {
		matched := false
		for _, p := range parent {
			if ScopeMatch(c, p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

