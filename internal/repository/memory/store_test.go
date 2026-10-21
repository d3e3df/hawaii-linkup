package memory

import (
	"errors"
	"slices"
	"testing"

	"hawaii-linkup/internal/domain"
)

func TestStore_LoadPair_MissingIsInitialState(t *testing.T) {
	s := NewStore()
	key := domain.NewPairKey("u1", "u2")

	got, err := s.LoadPair(key)

	if err != nil {
		t.Fatalf("отсутствие пары не должно быть ошибкой: %v", err)
	}
	if got != domain.NewPair(key) {
		t.Fatalf("ожидалось начальное состояние, got %+v", got)
	}
}

func TestStore_SaveThenLoad(t *testing.T) {
	s := NewStore()
	p := domain.NewPair(domain.NewPairKey("u1", "u2")).WithLike("u1")

	if err := s.SavePair(p); err != nil {
		t.Fatalf("SavePair: %v", err)
	}
	got, err := s.LoadPair(p.Key)

	if err != nil || got != p {
		t.Fatalf("got %+v, err %v; want %+v", got, err, p)
	}
}

func TestStore_LoadPair_KeyOrderDoesNotMatter(t *testing.T) {
	s := NewStore()
	p := domain.NewPair(domain.NewPairKey("u1", "u2")).WithLike("u2")
	_ = s.SavePair(p)

	got, _ := s.LoadPair(domain.NewPairKey("u2", "u1"))

	if got != p {
		t.Fatalf("(u2, u1) должна читать ту же пару: got %+v", got)
	}
}

func TestStore_SavePair_ReplacesExisting(t *testing.T) {
	s := NewStore()
	key := domain.NewPairKey("u1", "u2")
	_ = s.SavePair(domain.NewPair(key).WithLike("u1"))
	both := domain.NewPair(key).WithLike("u1").WithLike("u2")

	_ = s.SavePair(both)
	got, _ := s.LoadPair(key)

	if got != both {
		t.Fatalf("ожидалась замена состояния, got %+v", got)
	}
}

func TestStore_PairsAreIndependent(t *testing.T) {
	s := NewStore()
	_ = s.SavePair(domain.NewPair(domain.NewPairKey("u1", "u2")).WithLike("u1"))

	other, _ := s.LoadPair(domain.NewPairKey("u1", "u3"))

	if other.LikeCount() != 0 {
		t.Fatalf("запись одной пары изменила другую: %+v", other)
	}
}

func TestStore_PairIsolation(t *testing.T) {
	s := NewStore()
	key := domain.NewPairKey("u1", "u2")
	p := domain.NewPair(key).WithLike("u1")
	_ = s.SavePair(p)

	p.HighLiked = true

	again, _ := s.LoadPair(key)
	if again.HighLiked {
		t.Fatal("изменение без SavePair попало в хранилище")
	}
}

func TestStore_UserIsolation(t *testing.T) {
	s := NewStore()
	u := domain.User{ID: "u1", Interests: []string{"music"}, Active: true}
	s.AddUser(u)

	u.Interests[0] = "changed-arg"
	got, _ := s.GetUser("u1")
	got.Interests[0] = "changed-get"
	list, _ := s.ListUsers()
	list[0].Interests[0] = "changed-list"

	again, _ := s.GetUser("u1")
	if !slices.Equal(again.Interests, []string{"music"}) {
		t.Fatalf("интересы в хранилище изменились без записи: %v", again.Interests)
	}
}

func TestStore_LikesAreReadFromPairState(t *testing.T) {
	s := NewStore()
	_ = s.SavePair(domain.NewPair(domain.NewPairKey("u2", "u1")).WithLike("u2"))
	_ = s.SavePair(domain.NewPair(domain.NewPairKey("u3", "u4")).WithLike("u3"))

	liked, _ := s.HasLiked("u2", "u1")
	reverse, _ := s.HasLiked("u1", "u2")
	ids, _ := s.LikedIDs("u2")
	idsLow, _ := s.LikedIDs("u3")
	noneLow, _ := s.LikedIDs("u1")
	noneHigh, _ := s.LikedIDs("u4")

	if !liked || reverse {
		t.Fatalf("HasLiked не совпадает с состоянием пары: u2->u1=%v, u1->u2=%v", liked, reverse)
	}
	if !slices.Equal(ids, []string{"u1"}) {
		t.Fatalf("LikedIDs(u2) = %v, want [u1]", ids)
	}
	if !slices.Equal(idsLow, []string{"u4"}) {
		t.Fatalf("LikedIDs(u3) = %v, want [u4]", idsLow)
	}
	if len(noneLow) != 0 || len(noneHigh) != 0 {
		t.Fatalf("u1 и u4 никого не лайкали: LikedIDs(u1) = %v, LikedIDs(u4) = %v", noneLow, noneHigh)
	}
}

func TestStore_SaveLike_WritesToPair(t *testing.T) {
	s := NewStore()

	_ = s.SaveLike("u1", "u2")
	p, _ := s.LoadPair(domain.NewPairKey("u2", "u1"))

	if !p.HasLiked("u1") || p.HasLiked("u2") {
		t.Fatalf("SaveLike записал не то направление: %+v", p)
	}
}

func TestStore_InvalidKeyRejectedWithoutChanges(t *testing.T) {
	s := NewStore()
	good := domain.NewPair(domain.NewPairKey("u1", "u2")).WithLike("u1")
	_ = s.SavePair(good)
	wrong := domain.Pair{Key: domain.PairKey{Low: "u2", High: "u1"}, LowLiked: true, HighLiked: true}

	saveErr := s.SavePair(wrong)
	_, loadErr := s.LoadPair(wrong.Key)

	if !errors.Is(saveErr, domain.ErrInvalidPairKey) || !errors.Is(loadErr, domain.ErrInvalidPairKey) {
		t.Fatalf("ожидался ErrInvalidPairKey: save=%v, load=%v", saveErr, loadErr)
	}
	if got, _ := s.LoadPair(good.Key); got != good {
		t.Fatalf("невалидный вход изменил данные: %+v", got)
	}
}
