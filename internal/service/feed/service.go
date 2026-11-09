package feed

import (
	"errors"
	"fmt"

	"hawaii-linkup/internal/domain"
)

var ErrInvalidRequest = errors.New("invalid request")

const MaxLimit = 50

type Query struct {
	Interest string
	Limit    int
	Offset   int
}

type UserStorage interface {
	ListUsers() ([]domain.User, error)
	BlockedIDs(userID string) ([]string, error)
	LikedIDs(userID string) ([]string, error)
}

type Service struct {
	storage UserStorage
}

func NewService(storage UserStorage) *Service {
	return &Service{storage: storage}
}

func (s *Service) Feed(requesterID string, limit int) ([]domain.User, error) {
	return s.Page(requesterID, Query{Limit: limit})
}

func (s *Service) Page(requesterID string, query Query) ([]domain.User, error) {
	if requesterID == "" {
		return nil, fmt.Errorf("%w: пустой requesterID", ErrInvalidRequest)
	}
	if query.Limit < 1 || query.Limit > MaxLimit {
		return nil, fmt.Errorf("%w: limit должен быть от 1 до %d", ErrInvalidRequest, MaxLimit)
	}
	if query.Offset < 0 {
		return nil, fmt.Errorf("%w: offset должен быть неотрицательным", ErrInvalidRequest)
	}

	blocked, err := s.storage.BlockedIDs(requesterID)
	if err != nil {
		return nil, fmt.Errorf("feed: получение блокировок: %w", err)
	}
	liked, err := s.storage.LikedIDs(requesterID)
	if err != nil {
		return nil, fmt.Errorf("feed: получение лайков: %w", err)
	}
	users, err := s.storage.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("feed: получение пользователей: %w", err)
	}

	excluded := make([]string, 0, len(blocked)+len(liked))
	excluded = append(excluded, blocked...)
	excluded = append(excluded, liked...)

	return BuildPage(users, requesterID, excluded, query), nil
}
