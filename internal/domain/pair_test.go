package domain

import "testing"

func TestNewPairKey_SameForBothOrders(t *testing.T) {
	if NewPairKey("u1", "u2") != NewPairKey("u2", "u1") {
		t.Fatal("(u1, u2) и (u2, u1) должны давать один ключ")
	}
}

func TestPair_KeepsDirection(t *testing.T) {
	p := NewPair(NewPairKey("u2", "u1")).WithLike("u2")

	if !p.HasLiked("u2") {
		t.Fatal("лайк от u2 потерян")
	}
	if p.HasLiked("u1") {
		t.Fatal("u1 не лайкал, но HasLiked вернул true")
	}
}

func TestPair_IsMatchOnlyWhenBothLiked(t *testing.T) {
	p := NewPair(NewPairKey("u1", "u2"))

	if p.WithLike("u1").IsMatch() {
		t.Fatal("один лайк не match")
	}
	if !p.WithLike("u1").WithLike("u2").IsMatch() {
		t.Fatal("два лайка должны давать match")
	}
}

func TestPair_WithLikeDoesNotModifyOriginal(t *testing.T) {
	p := NewPair(NewPairKey("u1", "u2"))

	_ = p.WithLike("u1")

	if p.LikeCount() != 0 {
		t.Fatal("WithLike изменил исходную пару")
	}
}

func TestPair_WithLikeIgnoresStranger(t *testing.T) {
	p := NewPair(NewPairKey("u1", "u2"))

	if p.WithLike("u3") != p {
		t.Fatal("лайк от постороннего не должен менять пару")
	}
}

func TestPairKey_Valid(t *testing.T) {
	if !NewPairKey("u2", "u1").Valid() {
		t.Fatal("ключ из NewPairKey должен быть валидным")
	}
	if (PairKey{Low: "u2", High: "u1"}).Valid() {
		t.Fatal("ключ с обратным порядком должен быть невалидным")
	}
	if (PairKey{Low: "", High: "u1"}).Valid() {
		t.Fatal("ключ с пустым ID должен быть невалидным")
	}
}
