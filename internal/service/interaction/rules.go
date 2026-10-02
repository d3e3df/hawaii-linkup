package interaction

import "hawaii-linkup/internal/domain"

type Outcome string

const (
	OutcomeMatch    Outcome = "match"
	OutcomePending  Outcome = "pending"
	OutcomeRejected Outcome = "rejected"
)

// Decide определяет результат лайка from -> to
// reverseLike - лайкал ли to пользователя from раньше
func Decide(from, to domain.User, reverseLike bool) Outcome {
	if from.ID == to.ID || !from.Active || !to.Active {
		return OutcomeRejected
	}
	if reverseLike {
		return OutcomeMatch
	}
	return OutcomePending
}
