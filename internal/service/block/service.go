package block

import (
	"context"
	"errors"
	"fmt"

	domain "hawaii-linkup/internal/domain/models"
)

var (
	ErrInvalidRequest = errors.New("invalid block request")
	ErrSelfBlock      = errors.New("self block")
	ErrInactive       = errors.New("inactive profile")
)

type Result struct {
	Changed          bool
	Blocked          bool
	MatchDeactivated bool
}

type Storage interface {
	BlockUser(
		ctx context.Context,
		actorID, targetID string,
		validate func(actor, target domain.User, alreadyBlocked bool) error,
	) (changed, matchDeactivated bool, err error)
}

type Service struct {
	storage Storage
}

func NewService(storage Storage) *Service {
	return &Service{storage: storage}
}

func (s *Service) Block(ctx context.Context, actorID, targetID string) (Result, error) {
	if actorID == "" || targetID == "" {
		return Result{}, fmt.Errorf("%w: empty user ID", ErrInvalidRequest)
	}
	changed, deactivated, err := s.storage.BlockUser(ctx, actorID, targetID, Check)
	if err != nil {
		return Result{}, fmt.Errorf("block: %w", err)
	}
	return Result{Changed: changed, Blocked: true, MatchDeactivated: deactivated}, nil
}

func Check(actor, target domain.User, _ bool) error {
	if actor.ID == target.ID {
		return ErrSelfBlock
	}
	if !actor.Active || !target.Active {
		return ErrInactive
	}
	return nil
}
