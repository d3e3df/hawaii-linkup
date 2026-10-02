package memory

import "hawaii-linkup/internal/domain"

func NewDemoStore() *Store {
	s := NewStore()
	s.AddUser(domain.User{ID: "u1", Name: "Анна", Age: 25, City: "Москва", Interests: []string{"music", "travel"}, Active: true})
	s.AddUser(domain.User{ID: "u2", Name: "Борис", Age: 28, City: "Москва", Interests: []string{"music", "sport"}, Active: true})
	s.AddUser(domain.User{ID: "u3", Name: "Вера", Age: 31, City: "Казань", Interests: []string{"travel", "books"}, Active: true})
	s.AddUser(domain.User{ID: "u4", Name: "Глеб", Age: 22, City: "Москва", Interests: []string{"music"}, Active: true})
	s.AddUser(domain.User{ID: "u5", Name: "Дина", Age: 27, City: "Москва", Interests: []string{"music"}, Active: false})
	s.AddUser(domain.User{ID: "u6", Name: "Егор", Age: 35, City: "Москва", Interests: []string{"travel"}, Active: true})

	s.Block("u1", "u4")
	_ = s.SaveLike("u1", "u6")
	_ = s.SaveLike("u2", "u1")
	return s
}
