package reaction

import (
	"errors"
	"testing"

	"hawaii-linkup/internal/domain"
	"hawaii-linkup/internal/service/interaction"
)

type fakeStorage struct {
	users    map[string]domain.User
	pairs    map[domain.PairKey]domain.Pair
	blocked  bool
	getErr   map[string]error
	blockErr error
	loadErr  error
	saveErr  error
	saves    int
}

func newFake() *fakeStorage {
	inactive := user("x")
	inactive.Active = false
	return &fakeStorage{
		users: map[string]domain.User{
			"a": user("a"),
			"b": user("b"),
			"x": inactive,
		},
		pairs:  map[domain.PairKey]domain.Pair{},
		getErr: map[string]error{},
	}
}

func (f *fakeStorage) GetUser(id string) (domain.User, error) {
	if err := f.getErr[id]; err != nil {
		return domain.User{}, err
	}
	u, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeStorage) IsBlocked(a, b string) (bool, error) {
	return f.blocked, f.blockErr
}

func (f *fakeStorage) LoadPair(key domain.PairKey) (domain.Pair, error) {
	if f.loadErr != nil {
		return domain.Pair{}, f.loadErr
	}
	p, ok := f.pairs[key]
	if !ok {
		return domain.NewPair(key), nil
	}
	return p, nil
}

func (f *fakeStorage) SavePair(p domain.Pair) error {
	if f.saveErr != nil {
		return f.saveErr // отказ до изменения памяти
	}
	f.saves++
	f.pairs[p.Key] = p
	return nil
}

var errStorage = errors.New("storage is down")

var abKey = domain.NewPairKey("a", "b")

func TestLike_FirstLikeSaves(t *testing.T) {
	fake := newFake()

	got, err := NewService(fake).Like("a", "b")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	want := Result{Outcome: interaction.OutcomePending, Changed: true, Likes: 1}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if fake.saves != 1 || !fake.pairs[abKey].HasLiked("a") {
		t.Fatalf("лайк не сохранён: saves=%d, pair=%+v", fake.saves, fake.pairs[abKey])
	}
}

func TestLike_ResponseIsMatch(t *testing.T) {
	fake := newFake()
	fake.pairs[abKey] = domain.NewPair(abKey).WithLike("a")

	got, err := NewService(fake).Like("b", "a")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	want := Result{Outcome: interaction.OutcomeMatch, Changed: true, Likes: 2}
	if got != want || fake.saves != 1 {
		t.Fatalf("got %+v, saves=%d; want %+v, saves=1", got, fake.saves, want)
	}
}

func TestLike_RepeatDoesNotSave(t *testing.T) {
	fake := newFake()
	fake.pairs[abKey] = domain.NewPair(abKey).WithLike("a")

	got, err := NewService(fake).Like("a", "b")

	if err != nil {
		t.Fatalf("повтор не должен быть ошибкой: %v", err)
	}
	want := Result{Outcome: interaction.OutcomePending, Changed: false, Likes: 1}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if fake.saves != 0 {
		t.Fatalf("повтор не должен вызывать SavePair, вызовов: %d", fake.saves)
	}
}

func TestLike_NotAllowedDoesNotSave(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(f *fakeStorage)
		target string
	}{
		{"неактивный кандидат", func(f *fakeStorage) {}, "x"},
		{"блокировка", func(f *fakeStorage) { f.blocked = true }, "b"},
		{"сам себе", func(f *fakeStorage) {}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			tt.edit(fake)

			got, err := NewService(fake).Like("a", tt.target)

			if !errors.Is(err, ErrNotAllowed) {
				t.Fatalf("ожидался ErrNotAllowed, got %v", err)
			}
			if got != (Result{}) || fake.saves != 0 {
				t.Fatalf("при отказе: result=%+v, saves=%d; want нулевой результат и 0", got, fake.saves)
			}
		})
	}
}

func TestLike_UserNotFound(t *testing.T) {
	fake := newFake()

	_, err := NewService(fake).Like("a", "nobody")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидался domain.ErrNotFound, got %v", err)
	}
	if fake.saves != 0 {
		t.Fatal("при отсутствии пользователя запись не должна выполняться")
	}
}

func TestLike_EmptyID(t *testing.T) {
	_, err := NewService(newFake()).Like("", "b")

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ожидался ErrInvalidRequest, got %v", err)
	}
}

func TestLike_ReadErrorsStopBeforeSave(t *testing.T) {
	tests := []struct {
		name string
		edit func(f *fakeStorage)
	}{
		{"GetUser actor", func(f *fakeStorage) { f.getErr["a"] = errStorage }},
		{"GetUser target", func(f *fakeStorage) { f.getErr["b"] = errStorage }},
		{"IsBlocked", func(f *fakeStorage) { f.blockErr = errStorage }},
		{"LoadPair", func(f *fakeStorage) { f.loadErr = errStorage }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			tt.edit(fake)

			got, err := NewService(fake).Like("a", "b")

			if !errors.Is(err, errStorage) {
				t.Fatalf("причина ошибки потеряна: %v", err)
			}
			if got != (Result{}) || fake.saves != 0 {
				t.Fatalf("при ошибке чтения: result=%+v, saves=%d; want нулевой результат и 0", got, fake.saves)
			}
		})
	}
}

func TestLike_SaveErrorIsNotSuccess(t *testing.T) {
	fake := newFake()
	fake.saveErr = errStorage

	got, err := NewService(fake).Like("a", "b")

	if !errors.Is(err, errStorage) {
		t.Fatalf("причина ошибки потеряна: %v", err)
	}
	if got != (Result{}) {
		t.Fatalf("при ошибке записи нельзя сообщать успех, got %+v", got)
	}
	if _, saved := fake.pairs[abKey]; saved {
		t.Fatal("при отказе записи состояние не должно измениться")
	}
}
