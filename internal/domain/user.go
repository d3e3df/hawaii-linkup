package domain

import "errors"

var ErrNotFound = errors.New("not found")

type User struct {
	ID        string
	Name      string
	Age       int
	City      string
	Interests []string
	Active    bool
}
