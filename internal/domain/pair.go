package domain

import "errors"

var ErrInvalidPairKey = errors.New("invalid pair key")

type PairKey struct {
	Low  string
	High string
}

func NewPairKey(a, b string) PairKey {
	if a > b {
		a, b = b, a
	}
	return PairKey{Low: a, High: b}
}

func (k PairKey) Valid() bool {
	return k.Low != "" && k.High != "" && k.Low <= k.High
}

type Pair struct {
	Key       PairKey
	LowLiked  bool
	HighLiked bool
}

func NewPair(key PairKey) Pair {
	return Pair{Key: key}
}

func (p Pair) HasLiked(userID string) bool {
	switch userID {
	case p.Key.Low:
		return p.LowLiked
	case p.Key.High:
		return p.HighLiked
	default:
		return false
	}
}

func (p Pair) WithLike(userID string) Pair {
	switch userID {
	case p.Key.Low:
		p.LowLiked = true
	case p.Key.High:
		p.HighLiked = true
	}
	return p
}

func (p Pair) IsMatch() bool {
	return p.LowLiked && p.HighLiked
}

func (p Pair) LikeCount() int {
	count := 0
	if p.LowLiked {
		count++
	}
	if p.HighLiked {
		count++
	}
	return count
}
