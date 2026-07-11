package respin

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRunnerTryAcquireBoundsConcurrency(t *testing.T) {
	r := NewRunner(2, nil)

	s1, ok := r.TryAcquire()
	if !ok {
		t.Fatal("first acquire should succeed")
	}
	s2, ok := r.TryAcquire()
	if !ok {
		t.Fatal("second acquire should succeed")
	}
	if _, ok := r.TryAcquire(); ok {
		t.Fatal("third acquire should fail at capacity")
	}

	s1.Release()
	s3, ok := r.TryAcquire()
	if !ok {
		t.Fatal("acquire after release should succeed")
	}
	s2.Release()
	s3.Release()
}

func TestRunnerStartFansOutAndReplays(t *testing.T) {
	r := NewRunner(1, nil)
	slot, _ := r.TryAcquire()

	release := make(chan struct{})
	done := make(chan struct{})
	r.Start("imp-1", func(ctx context.Context, sink ProgressSink) (RunResult, error) {
		sink.Status(StatusFetching)
		<-release // hold the run open so we can subscribe mid-flight
		sink.Status(StatusSucceeded)
		close(done)
		return RunResult{Status: StatusSucceeded, SiteID: "site-1"}, nil
	}, slot)

	// Wait until the first status event lands in the buffer, then subscribe: a
	// late subscriber must still receive the replayed history.
	waitFor(t, func() bool {
		ch, ok := r.Subscribe("imp-1")
		if !ok {
			return false
		}
		select {
		case ev := <-ch:
			return ev.Type == EventStatus && ev.Status == StatusFetching
		default:
			return false
		}
	})

	ch, ok := r.Subscribe("imp-1")
	if !ok {
		t.Fatal("subscribe should find the active run")
	}
	close(release)
	<-done

	var events []Event
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev, open := <-ch:
			if !open {
				assertTerminalComplete(t, events)
				return
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatal("timed out waiting for terminal event")
		}
	}
}

func TestRunnerSubscribeUnknownRun(t *testing.T) {
	r := NewRunner(1, nil)
	if _, ok := r.Subscribe("missing"); ok {
		t.Fatal("subscribe to unknown run should report no active run")
	}
}

func assertTerminalComplete(t *testing.T, events []Event) {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("expected replayed events")
	}
	last := events[len(events)-1]
	if last.Type != EventComplete {
		t.Fatalf("expected terminal complete event, got %q", last.Type)
	}
	if last.Result == nil || last.Result.SiteID != "site-1" {
		t.Fatalf("expected result with site-1, got %+v", last.Result)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

// exercise concurrent publishes to catch races under -race.
func TestRunnerConcurrentSubscribers(t *testing.T) {
	r := NewRunner(1, nil)
	slot, _ := r.TryAcquire()
	start := make(chan struct{})
	r.Start("imp-x", func(ctx context.Context, sink ProgressSink) (RunResult, error) {
		<-start
		for i := 0; i < 20; i++ {
			sink.Status(StatusExtracting)
		}
		return RunResult{Status: StatusSucceeded}, nil
	}, slot)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ch, ok := r.Subscribe("imp-x"); ok {
				for range ch {
				}
			}
		}()
	}
	close(start)
	wg.Wait()
}
