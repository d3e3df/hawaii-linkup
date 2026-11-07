package memory

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"hawaii-linkup/internal/domain"
)

type Store struct {
	mu     sync.RWMutex
	users  []domain.User
	blocks map[string]map[string]bool
	pairs  map[domain.PairKey]domain.Pair
}

func NewStore() *Store {
	return &Store{
		blocks: make(map[string]map[string]bool),
		pairs:  make(map[domain.PairKey]domain.Pair),
	}
}

func (s *Store) AddUser(u domain.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = append(s.users, cloneUser(u))
}

func (s *Store) Block(who, whom string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blocks[who] == nil {
		s.blocks[who] = make(map[string]bool)
	}
	s.blocks[who][whom] = true
}

func (s *Store) ListUsers() ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.User, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, cloneUser(u))
	}
	return result, nil
}

func (s *Store) GetUser(id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getUser(id)
}

func (s *Store) getUser(id string) (domain.User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return cloneUser(u), nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *Store) IsBlocked(a, b string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isBlocked(a, b), nil
}

func (s *Store) isBlocked(a, b string) bool {
	return s.blocks[a][b] || s.blocks[b][a]
}

func (s *Store) BlockedIDs(userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

func (s *Store) LoadPair(key domain.PairKey) (domain.Pair, error) {
	if !key.Valid() {
		return domain.Pair{}, fmt.Errorf("memory: %w: %+v", domain.ErrInvalidPairKey, key)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadPair(key), nil
}

func (s *Store) loadPair(key domain.PairKey) domain.Pair {
	p, ok := s.pairs[key]
	if !ok {
		return domain.NewPair(key)
	}
	return p
}

func (s *Store) SavePair(p domain.Pair) error {
	if !p.Key.Valid() {
		return fmt.Errorf("memory: %w: %+v", domain.ErrInvalidPairKey, p.Key)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pairs[p.Key] = p
	return nil
}

func (s *Store) UpdatePair(
	key domain.PairKey,
	otherID string,
	update func(current domain.Pair, blocked bool) (domain.Pair, error),
) (domain.Pair, error) {
	if !key.Valid() {
		return domain.Pair{}, fmt.Errorf("memory: %w: %+v", domain.ErrInvalidPairKey, key)
	}
	if otherID != key.Low && otherID != key.High {
		return domain.Pair{}, fmt.Errorf("memory: %w: other user is outside pair", domain.ErrInvalidPairKey)
	}
	if update == nil {
		return domain.Pair{}, errors.New("memory: nil pair update")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.loadPair(key)
	next, err := update(current, s.isBlocked(key.Low, key.High))
	if err != nil {
		return domain.Pair{}, err
	}
	if next.Key != key {
		return domain.Pair{}, fmt.Errorf("memory: %w: update changed key", domain.ErrInvalidPairKey)
	}
	if next != current {
		s.pairs[key] = next
	}
	return next, nil
}

func (s *Store) SaveLike(from, to string) error {
	_, err := s.UpdatePair(domain.NewPairKey(from, to), to, func(current domain.Pair, _ bool) (domain.Pair, error) {
		return current.WithLike(from), nil
	})
	return err
}

func (s *Store) HasLiked(from, to string) (bool, error) {
	key := domain.NewPairKey(from, to)
	if !key.Valid() {
		return false, fmt.Errorf("memory: %w: %+v", domain.ErrInvalidPairKey, key)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadPair(key).HasLiked(from), nil
}

func (s *Store) LikedIDs(userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := []string{}
	for key, p := range s.pairs {
		if key.Low == userID && p.LowLiked {
			ids = append(ids, key.High)
		}
		if key.High == userID && p.HighLiked {
			ids = append(ids, key.Low)
		}
	}
	return ids, nil
}

func cloneUser(u domain.User) domain.User {
	u.Interests = slices.Clone(u.Interests)
	return u
}
