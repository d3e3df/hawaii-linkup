//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	domain "hawaii-linkup/internal/domain/models"
	"hawaii-linkup/internal/service/block"
	"hawaii-linkup/internal/service/feed"
	"hawaii-linkup/internal/service/reaction"
)

func TestWithdrawSuccessRepeatAndRepeatedLike(t *testing.T) {
	store, reactions := newReactionService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	createMatchedPair(t, ctx, reactions, "u2", "u4")
	first, err := reactions.Withdraw(ctx, "u2", "u4")
	if err != nil || !first.Changed || first.Likes != 1 || first.Matched || !first.MatchDeactivated {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	pair, err := store.LoadPairContext(ctx, domain.NewPairKey("u2", "u4"))
	if err != nil || pair.HasLiked("u2") || !pair.HasLiked("u4") || pair.HasActiveMatch() {
		t.Fatalf("pair=%+v err=%v", pair, err)
	}
	repeated, err := reactions.Withdraw(ctx, "u2", "u4")
	if err != nil || repeated.Changed || repeated.Likes != 1 || repeated.MatchDeactivated {
		t.Fatalf("repeat=%+v err=%v", repeated, err)
	}
	users, err := feed.NewService(store).PageContext(ctx, "u2", feed.Query{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, user := range users {
		found = found || user.ID == "u4"
	}
	if !found {
		t.Fatal("withdrawn target did not return to discovery")
	}
	likedAgain, err := reactions.Like(ctx, "u2", "u4")
	if err != nil || !likedAgain.Changed || !likedAgain.Matched || !likedAgain.MatchedNow {
		t.Fatalf("like again=%+v err=%v", likedAgain, err)
	}
}

func TestWithdrawRollsBackAfterIntermediateDelete(t *testing.T) {
	store, reactions := newReactionService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	createMatchedPair(t, ctx, reactions, "u2", "u4")
	if _, err := store.pool.Exec(ctx, `CREATE FUNCTION fail_withdraw_match_update() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced withdrawal match failure'; END;
	$$`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, "CREATE TRIGGER fail_withdraw BEFORE UPDATE ON matches FOR EACH ROW EXECUTE FUNCTION fail_withdraw_match_update()"); err != nil {
		t.Fatal(err)
	}
	if _, err := reactions.Withdraw(ctx, "u2", "u4"); err == nil {
		t.Fatal("expected forced match update failure")
	}
	verification, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer verification.Release()
	var reactionPresent, activeMatch bool
	if err := verification.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM reactions WHERE actor_id = 'u2' AND target_id = 'u4')").Scan(&reactionPresent); err != nil {
		t.Fatal(err)
	}
	if err := verification.QueryRow(ctx, "SELECT active FROM matches WHERE low_id = 'u2' AND high_id = 'u4'").Scan(&activeMatch); err != nil {
		t.Fatal(err)
	}
	if !reactionPresent || !activeMatch {
		t.Fatalf("reaction=%t active match=%t", reactionPresent, activeMatch)
	}
}

type operationResponse struct {
	like     reaction.Result
	withdraw reaction.WithdrawResult
	block    block.Result
	err      error
}

func TestConcurrentLikeAndWithdrawSerializeOnPairLock(t *testing.T) {
	t.Run("like first creates match then withdrawal deactivates it", func(t *testing.T) {
		store, reactions := newReactionService(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := reactions.Like(ctx, "u2", "u4"); err != nil {
			t.Fatal(err)
		}
		barrier := newTestBarrier()
		store.afterPairLock = barrier.hook
		defer barrier.releaseNow()
		firstDone := make(chan operationResponse, 1)
		secondDone := make(chan operationResponse, 1)
		go func() {
			result, err := reactions.Like(ctx, "u4", "u2")
			firstDone <- operationResponse{like: result, err: err}
		}()
		waitSignal(t, barrier.entered)
		go func() {
			result, err := reactions.Withdraw(ctx, "u2", "u4")
			secondDone <- operationResponse{withdraw: result, err: err}
		}()
		waitForLockWaiter(t, store)
		barrier.releaseNow()
		liked := <-firstDone
		withdrawn := <-secondDone
		if liked.err != nil || !liked.like.MatchedNow || withdrawn.err != nil || !withdrawn.withdraw.MatchDeactivated {
			t.Fatalf("like=%+v withdraw=%+v errors=%v / %v", liked.like, withdrawn.withdraw, liked.err, withdrawn.err)
		}
		assertWithdrawnPair(t, ctx, store, "u2", "u4", "u4")
	})

	t.Run("withdrawal first prevents reciprocal match", func(t *testing.T) {
		store, reactions := newReactionService(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := reactions.Like(ctx, "u2", "u4"); err != nil {
			t.Fatal(err)
		}
		barrier := newTestBarrier()
		store.afterPairLock = barrier.hook
		defer barrier.releaseNow()
		firstDone := make(chan operationResponse, 1)
		secondDone := make(chan operationResponse, 1)
		go func() {
			result, err := reactions.Withdraw(ctx, "u2", "u4")
			firstDone <- operationResponse{withdraw: result, err: err}
		}()
		waitSignal(t, barrier.entered)
		go func() {
			result, err := reactions.Like(ctx, "u4", "u2")
			secondDone <- operationResponse{like: result, err: err}
		}()
		waitForLockWaiter(t, store)
		barrier.releaseNow()
		withdrawn := <-firstDone
		liked := <-secondDone
		if withdrawn.err != nil || !withdrawn.withdraw.Changed || withdrawn.withdraw.MatchDeactivated || liked.err != nil || liked.like.Matched {
			t.Fatalf("withdraw=%+v like=%+v errors=%v / %v", withdrawn.withdraw, liked.like, withdrawn.err, liked.err)
		}
		assertWithdrawnPair(t, ctx, store, "u2", "u4", "u4")
	})
}

func TestConcurrentWithdrawAndBlockSerializeOnPairLock(t *testing.T) {
	t.Run("withdrawal first then block", func(t *testing.T) {
		store, reactions := newReactionService(t)
		blocks := block.NewService(store)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		createMatchedPair(t, ctx, reactions, "u2", "u4")
		barrier := newTestBarrier()
		store.afterPairLock = barrier.hook
		defer barrier.releaseNow()
		firstDone := make(chan operationResponse, 1)
		secondDone := make(chan operationResponse, 1)
		go func() {
			result, err := reactions.Withdraw(ctx, "u2", "u4")
			firstDone <- operationResponse{withdraw: result, err: err}
		}()
		waitSignal(t, barrier.entered)
		go func() {
			result, err := blocks.Block(ctx, "u4", "u2")
			secondDone <- operationResponse{block: result, err: err}
		}()
		waitForLockWaiter(t, store)
		barrier.releaseNow()
		withdrawn := <-firstDone
		blocked := <-secondDone
		if withdrawn.err != nil || !withdrawn.withdraw.MatchDeactivated || blocked.err != nil || blocked.block.MatchDeactivated {
			t.Fatalf("withdraw=%+v block=%+v errors=%v / %v", withdrawn.withdraw, blocked.block, withdrawn.err, blocked.err)
		}
		assertWithdrawnPair(t, ctx, store, "u2", "u4", "u4")
	})

	t.Run("block first rejects waiting withdrawal", func(t *testing.T) {
		store, reactions := newReactionService(t)
		blocks := block.NewService(store)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		createMatchedPair(t, ctx, reactions, "u2", "u4")
		barrier := newTestBarrier()
		store.afterPairLock = barrier.hook
		defer barrier.releaseNow()
		firstDone := make(chan operationResponse, 1)
		secondDone := make(chan operationResponse, 1)
		go func() {
			result, err := blocks.Block(ctx, "u4", "u2")
			firstDone <- operationResponse{block: result, err: err}
		}()
		waitSignal(t, barrier.entered)
		go func() {
			result, err := reactions.Withdraw(ctx, "u2", "u4")
			secondDone <- operationResponse{withdraw: result, err: err}
		}()
		waitForLockWaiter(t, store)
		barrier.releaseNow()
		blocked := <-firstDone
		withdrawn := <-secondDone
		if blocked.err != nil || !blocked.block.MatchDeactivated {
			t.Fatalf("block=%+v err=%v", blocked.block, blocked.err)
		}
		assertBlockedReaction(t, withdrawn.err)
		pair, err := store.LoadPairContext(ctx, domain.NewPairKey("u2", "u4"))
		if err != nil || pair.LikeCount() != 2 || pair.HasActiveMatch() {
			t.Fatalf("pair=%+v err=%v", pair, err)
		}
	})
}

func createMatchedPair(t *testing.T, ctx context.Context, reactions *reaction.Service, actorID, targetID string) {
	t.Helper()
	if _, err := reactions.Like(ctx, actorID, targetID); err != nil {
		t.Fatal(err)
	}
	if _, err := reactions.Like(ctx, targetID, actorID); err != nil {
		t.Fatal(err)
	}
}

func assertWithdrawnPair(t *testing.T, ctx context.Context, store *Store, actorID, targetID, remainingActor string) {
	t.Helper()
	pair, err := store.LoadPairContext(ctx, domain.NewPairKey(actorID, targetID))
	if err != nil || pair.LikeCount() != 1 || !pair.HasLiked(remainingActor) || pair.HasActiveMatch() {
		t.Fatalf("pair=%+v err=%v", pair, err)
	}
}
