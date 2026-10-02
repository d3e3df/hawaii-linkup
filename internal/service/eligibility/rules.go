package eligibility

import "hawaii-linkup/internal/domain"

type Reason string

const (
	ReasonOK           Reason = "ok"
	ReasonSelf         Reason = "self"
	ReasonInactive     Reason = "inactive"
	ReasonBlocked      Reason = "blocked"
	ReasonAlreadyLiked Reason = "already_liked"
)

type Decision struct {
	Allowed bool
	Reason  Reason
}

func Check(from, to domain.User, blocked, alreadyLiked bool) Decision {
	switch {
	case from.ID == to.ID:
		return Decision{Allowed: false, Reason: ReasonSelf}
	case !from.Active || !to.Active:
		return Decision{Allowed: false, Reason: ReasonInactive}
	case blocked:
		return Decision{Allowed: false, Reason: ReasonBlocked}
	case alreadyLiked:
		return Decision{Allowed: false, Reason: ReasonAlreadyLiked}
	default:
		return Decision{Allowed: true, Reason: ReasonOK}
	}
}
