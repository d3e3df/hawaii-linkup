package reaction

import (
	"errors"
	"fmt"

	"hawaii-linkup/internal/domain"
	"hawaii-linkup/internal/service/eligibility"
	"hawaii-linkup/internal/service/interaction"
)

var (
	ErrNotAllowed   = errors.New("like not allowed")
	ErrPairMismatch = errors.New("pair does not match users")
)

// Decision — результат чистого перехода
type Decision struct {
	Pair    domain.Pair
	Outcome interaction.Outcome
	Changed bool
}

// Transition решает, что происходит, когда actor лайкает target при текущем состоянии пары
// blocked — есть ли блокировка между ними в любую сторону
func Transition(current domain.Pair, actor, target domain.User, blocked bool) (Decision, error) {
	if current.Key != domain.NewPairKey(actor.ID, target.ID) {
		return Decision{}, ErrPairMismatch
	}

	check := eligibility.Check(actor, target, blocked, false)
	if !check.Allowed {
		return Decision{}, fmt.Errorf("%w: %s", ErrNotAllowed, check.Reason)
	}

	outcome := interaction.Decide(actor, target, current.HasLiked(target.ID))

	if current.HasLiked(actor.ID) {
		return Decision{Pair: current, Outcome: outcome, Changed: false}, nil
	}
	return Decision{Pair: current.WithLike(actor.ID), Outcome: outcome, Changed: true}, nil
}
