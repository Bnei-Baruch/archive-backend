package events

import (
	"context"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

type WorkQueue interface {
	Init()
	Close()
	Enqueue(WorkTask)
}

type WorkTask interface {
	Do()
}

type IndexerQueue struct {
	jobs   chan WorkTask
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func (q *IndexerQueue) Init() {
	q.jobs = make(chan WorkTask, 10000)
	q.ctx, q.cancel = context.WithCancel(context.Background())

	// Start worker here.
	go func() {
		defer q.wg.Done()

		for {
			select {
			case <-q.ctx.Done():
				log.Infof("IndexerQueue.Init(%d) - Worker: ctx.Done.", len(q.jobs))
				return

			case job := <-q.jobs:
				log.Infof("IndexerQueue.Dequeue(%d) - Task was taken from the queue: %+v", len(q.jobs), job)
				job.Do()
				if q.ctx.Err() != nil {
					log.Infof("IndexerQueue.Init(%d) - Worker: Context cancelled.", len(q.jobs))
					return
				}
			}
		}
	}()

	// Add the worker to the waiting group.
	q.wg.Add(1)
}

func (q *IndexerQueue) Close() {
	log.Infof("IndexerQueue.Close(%d) - Close jobs channel.", len(q.jobs))
	close(q.jobs)

	log.Info("IndexerQueue.Close(closed) - Cancel worker context.")
	q.cancel()

	log.Info("IndexerQueue.Close(closed) - Wait for worker to finish.")
	if !WaitTimeout(&q.wg, 5*time.Second) {
		log.Warn("IndexerQueue.Close(closed) - WaitGroup closed by timeout.")
	}
}

func (q *IndexerQueue) Enqueue(task WorkTask) {
	for {
		select {
		case <-time.After(2 * time.Second):
			log.Warnf("IndexerQueue.Enqueue(%d) - Enqueue timeout.", len(q.jobs))
		case q.jobs <- task:
			log.Infof("IndexerQueue.Enqueue(%d) - Task was added to queue: %+v", len(q.jobs), task)
			return
		}
	}
}

type IndexerTask struct {
	F func(s string) error
	S string
}

func (t IndexerTask) Do() {
	clock := time.Now()

	fp := strings.Split(runtime.FuncForPC(reflect.ValueOf(t.F).Pointer()).Name(), ".")
	fName := fp[len(fp)-1]

	// Don't panic !
	defer func() {
		if rval := recover(); rval != nil {
			log.Errorf("IndexerTask.Do - %s panic: %s", fName, rval)
			debug.PrintStack()
		}
	}()

	err := t.F(t.S)
	log.Infof("IndexerTask.Do - %s took %s", fName, time.Now().Sub(clock).String())
	if err != nil {
		log.Errorf("IndexerTask.Do - %s: %s", fName, err.Error())
	}
}

// WaitTimeout does a Wait on a sync.WaitGroup object but with a specified
// timeout. Returns true if the wait completed without timing out, false
// otherwise.
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}
