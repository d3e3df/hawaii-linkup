package models

type User struct {
	ID        string
	Name      string
	Age       int
	City      string
	Interests []string
	Active    bool
	Version   int
}

type ProfileUpdate struct {
	Name      string
	Age       int
	City      string
	Interests []string
	Version   int
}
