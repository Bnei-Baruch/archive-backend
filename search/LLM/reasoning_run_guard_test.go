package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReasoningRunGuardLimitsAndResets(t *testing.T) {
	g := NewReasoningRunGuard()
	g.maxConcurrent, g.maxIP, g.maxDaily = 1, 2, 3
	run, retry := g.TryStart(context.Background(), "one")
	require.NotNil(t, run)
	run.Commit()
	require.Zero(t, retry)
	deadline, ok := run.Context.Deadline()
	require.True(t, ok)
	require.InDelta(t, defaultReasoningRunTimeout.Seconds(), time.Until(deadline).Seconds(), 1)
	blocked, retry := g.TryStart(context.Background(), "two")
	require.Nil(t, blocked)
	require.Positive(t, retry)
	require.NotContains(t, g.startsByIP, "two", "rejections must not grow the IP map")
	run.Finish()
	run.Finish()
	require.Zero(t, g.active)
	run, _ = g.TryStart(context.Background(), "one")
	require.NotNil(t, run)
	run.Commit()
	run.Finish()
	blocked, retry = g.TryStart(context.Background(), "one")
	require.Nil(t, blocked)
	require.True(t, retry > 0 && retry <= 60)
	g.minute = time.Time{}
	run, _ = g.TryStart(context.Background(), "one")
	require.NotNil(t, run)
	run.Commit()
	run.Finish()
	blocked, retry = g.TryStart(context.Background(), "different")
	require.Nil(t, blocked)
	require.True(t, retry > 0 && retry <= 86400)
	g.day = time.Time{}
	run, _ = g.TryStart(context.Background(), "different")
	require.NotNil(t, run)
	run.Commit()
	run.Finish()
	require.Equal(t, 1, g.daily)
}

func TestReasoningRunGuardRefundsPreparationFailureButNotIPAttempt(t *testing.T) {
	g := NewReasoningRunGuard()
	g.maxDaily, g.maxIP = 1, 1
	run, _ := g.TryStart(context.Background(), "one")
	require.NotNil(t, run)
	run.Finish()
	run.Finish()
	require.Zero(t, g.daily)
	require.Zero(t, g.active)
	blocked, _ := g.TryStart(context.Background(), "one")
	require.Nil(t, blocked, "failed preparation must still count toward the IP limit")
	run, _ = g.TryStart(context.Background(), "two")
	require.NotNil(t, run, "failed preparation must not consume the daily allowance")
	run.Commit()
	run.Finish()
	require.Equal(t, 1, g.daily)
}

func TestReasoningRunGuardDoesNotRefundAnotherDay(t *testing.T) {
	g := NewReasoningRunGuard()
	old, _ := g.TryStart(context.Background(), "old")
	// Simulate a reservation that outlived the UTC day in which it was made.
	old.day = old.day.Add(-24 * time.Hour)
	g.daily = 0
	current, _ := g.TryStart(context.Background(), "current")
	current.Commit()
	old.Finish()
	require.Equal(t, 1, g.daily)
	current.Finish()
	require.Equal(t, 1, g.daily)
}

func TestReasoningRunGuardConcurrentAdmission(t *testing.T) {
	g := NewReasoningRunGuard()
	g.maxConcurrent, g.maxDaily = 7, 1000
	runs := make(chan *ReasoningRun, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if run, _ := g.TryStart(context.Background(), fmt.Sprint(i)); run != nil {
				runs <- run
			}
		}(i)
	}
	wg.Wait()
	close(runs)
	require.Len(t, runs, 7)
	require.Equal(t, 7, g.daily)
	require.Len(t, g.startsByIP, 7)
	for run := range runs {
		run.Finish()
	}
	require.Zero(t, g.active)
}

func TestReasoningRunGuardDeadlineAndWorkerCleanup(t *testing.T) {
	g := NewReasoningRunGuard()
	g.maxConcurrent, g.timeout = 1, 20*time.Millisecond
	run, _ := g.TryStart(context.Background(), "one")
	done, ok := TrackReasoningRunWorker(run.Context)
	require.True(t, ok)
	select {
	case <-run.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("run deadline did not fire")
	}
	require.ErrorIs(t, run.Context.Err(), context.DeadlineExceeded)
	finished := make(chan struct{})
	go func() { run.Finish(); close(finished) }()
	_, ok = TrackReasoningRunWorker(run.Context)
	require.False(t, ok, "late callbacks must not start new work")
	blocked, _ := g.TryStart(context.Background(), "two")
	require.Nil(t, blocked, "deadline must not free capacity before workers exit")
	done()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("run did not release its slot after worker exit")
	}
	next, _ := g.TryStart(context.Background(), "two")
	require.NotNil(t, next)
	next.Finish()
}

func TestReasoningRunGuardToolPanicReleasesWorker(t *testing.T) {
	g := NewReasoningRunGuard()
	run, _ := g.TryStart(context.Background(), "one")
	_, err := ExecuteReasoningToolExecutions(run.Context, []ReasoningToolExecution{{Name: "broken"}}, map[string]ToolHandler{
		"broken": func(context.Context, json.RawMessage) (string, error) { panic("tool panic") },
	}, nil)
	require.Error(t, err)
	finished := make(chan struct{})
	go func() { run.Finish(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("panicking tool leaked a tracked worker")
	}
	require.Zero(t, g.active)
}
