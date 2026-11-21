package llm

import (
	"context"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

// Process-local safeguards; no external configuration is required.
const (
	defaultReasoningMaxConcurrentRuns = 30
	defaultReasoningStartsPerIP       = 60
	defaultReasoningStartsPerDay      = 5000
	defaultReasoningRunTimeout        = 5 * time.Minute
)

type ReasoningRunGuard struct {
	mu            sync.Mutex
	active, daily int
	day, minute   time.Time
	startsByIP    map[string]int
	maxConcurrent int
	maxIP         int
	maxDaily      int
	timeout       time.Duration
}

func NewReasoningRunGuard() *ReasoningRunGuard {
	return &ReasoningRunGuard{
		maxConcurrent: defaultReasoningMaxConcurrentRuns,
		maxIP:         defaultReasoningStartsPerIP,
		maxDaily:      defaultReasoningStartsPerDay,
		timeout:       defaultReasoningRunTimeout,
	}
}

// TryStart reserves admission before any workflow state or paid work is created.
// Rejections return Retry-After in seconds. The daily reservation is refunded
// unless Commit is called; failed preparation still counts toward the IP limit.
func (g *ReasoningRunGuard) TryStart(parent context.Context, ip string) (*ReasoningRun, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().UTC()
	day := now.Truncate(24 * time.Hour)
	minute := now.Truncate(time.Minute)
	if !g.day.Equal(day) {
		g.day, g.daily = day, 0
	}
	if !g.minute.Equal(minute) {
		g.minute = minute
		g.startsByIP = make(map[string]int)
	}
	if g.daily >= g.maxDaily {
		log.Warnf("Reasoning run rejected: limit=daily ip=%s active=%d daily=%d max=%d", ip, g.active, g.daily, g.maxDaily)
		return nil, int(day.Add(24*time.Hour).Sub(now)/time.Second) + 1
	}
	if g.startsByIP[ip] >= g.maxIP {
		log.Warnf("Reasoning run rejected: limit=ip ip=%s active=%d daily=%d ip_starts=%d max=%d", ip, g.active, g.daily, g.startsByIP[ip], g.maxIP)
		return nil, int(minute.Add(time.Minute).Sub(now)/time.Second) + 1
	}
	if g.active >= g.maxConcurrent {
		log.Warnf("Reasoning run rejected: limit=concurrent ip=%s active=%d daily=%d max=%d", ip, g.active, g.daily, g.maxConcurrent)
		return nil, 5
	}
	g.active++
	g.daily++
	g.startsByIP[ip]++
	ctx, cancel := context.WithTimeout(parent, g.timeout)
	run := &ReasoningRun{guard: g, cancel: cancel, day: day}
	run.Context = context.WithValue(ctx, reasoningRunContextKey{}, run)
	return run, 0
}

type reasoningRunContextKey struct{}

type ReasoningRun struct {
	Context   context.Context
	guard     *ReasoningRunGuard
	cancel    context.CancelFunc
	mu        sync.Mutex
	closing   bool
	day       time.Time
	committed bool
	workers   sync.WaitGroup
	once      sync.Once
}

func (r *ReasoningRun) Cancel() { r.cancel() }

// Commit keeps the daily charge once session preparation has succeeded.
func (r *ReasoningRun) Commit() {
	r.mu.Lock()
	if !r.closing {
		r.committed = true
	}
	r.mu.Unlock()
}

// Finish cancels and joins local workers before releasing capacity. Cancellation
// alone does not release a slot while a provider/tool call is still exiting.
func (r *ReasoningRun) Finish() {
	r.once.Do(func() {
		r.mu.Lock()
		r.closing = true
		committed := r.committed
		r.cancel()
		r.mu.Unlock()
		r.workers.Wait()
		r.guard.mu.Lock()
		r.guard.active--
		// An old run must not refund a reservation from a new UTC day.
		if !committed && r.guard.day.Equal(r.day) {
			r.guard.daily--
		}
		r.guard.mu.Unlock()
	})
}

// TrackReasoningRunWorker prevents late callbacks from starting new work during
// cleanup. Unguarded callers (e.g. tests) retain their existing behavior.
func TrackReasoningRunWorker(ctx context.Context) (done func(), ok bool) {
	run, _ := ctx.Value(reasoningRunContextKey{}).(*ReasoningRun)
	if run == nil {
		return func() {}, true
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.closing || ctx.Err() != nil {
		return nil, false
	}
	run.workers.Add(1)
	return run.workers.Done, true
}
