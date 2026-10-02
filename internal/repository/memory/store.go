package memory

import (
	"slices"

	"hawaii-linkup/internal/domain"
)

type Store struct {
	users  []domain.User
	blocks map[string]map[string]bool
	likes  map[string]map[string]bool
}

func NewStore() *Store {
	return &Store{
		blocks: make(map[string]map[string]bool),
		likes:  make(map[string]map[string]bool),
	}
}

// AddUser добавляет пользователя
func (s *Store) AddUser(u domain.User) {
	s.users = append(s.users, cloneUser(u))
}

// Block фиксирует, что who заблокировал whom
func (s *Store) Block(who, whom string) {
	if s.blocks[who] == nil {
		s.blocks[who] = make(map[string]bool)
	}
	s.blocks[who][whom] = true
}

// ListUsers возвращает копию списка всех пользователей
func (s *Store) ListUsers() ([]domain.User, error) {
	result := make([]domain.User, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, cloneUser(u))
	}
	return result, nil
}

// GetUser возвращает пользователя по ID или domain.ErrNotFound
func (s *Store) GetUser(id string) (domain.User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return cloneUser(u), nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

// IsBlocked сообщает, есть ли блокировка между a и b в любую сторону
func (s *Store) IsBlocked(a, b string) (bool, error) {
	return s.blocks[a][b] || s.blocks[b][a], nil
}

// BlockedIDs возвращает ID тех, кого заблокировал userID, и тех, кто заблокировал userID
func (s *Store) BlockedIDs(userID string) ([]string, error) {
	ids := []string{}
	for whom := range s.blocks[userID] {
		ids = append(ids, whom)
	}
	for who, set := range s.blocks {
		if set[userID] {
			ids = append(ids, who)
		}
	}
	return ids, nil
}

// SaveLike сохраняет лайк from -> to
func (s *Store) SaveLike(from, to string) error {
	if s.likes[from] == nil {
		s.likes[from] = make(map[string]bool)
	}
	s.likes[from][to] = true
	return nil
}

// HasLiked сообщает, лайкал ли from пользователя to
func (s *Store) HasLiked(from, to string) (bool, error) {
	return s.likes[from][to], nil
}

// LikedIDs возвращает ID всех, кого лайкнул userID
func (s *Store) LikedIDs(userID string) ([]string, error) {
	ids := []string{}
	for id := range s.likes[userID] {
		ids = append(ids, id)
	}
	return ids, nil
}

// cloneUser копирует пользователя вместе со срезом интересов
func cloneUser(u domain.User) domain.User {
	u.Interests = slices.Clone(u.Interests)
	return u
}
