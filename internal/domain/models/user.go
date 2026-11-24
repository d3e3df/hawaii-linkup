package models

type User struct {
	ID        string
	Name      string
	Age       int
	City      string
	Interests []string
	Active    bool
}
