package events

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"runtime/debug"
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

	// start worker here
	go func() {
		defer q.wg.Done()

		for {
			select {
			case <-q.ctx.Done():
				log.Info("worker: ctx.Done")
				return

			case job := <-q.jobs:
				job.Do()
				if q.ctx.Err() != nil {
					log.Info("worker: context cancelled")
					return
				}
			}
		}
	}()

	// add the worker to the waiting group
	q.wg.Add(1)
}

func (q *IndexerQueue) Close() {
	log.Info("close jobs channel")
	close(q.jobs)

	log.Info("cancel worker context")
	q.cancel()

	log.Info("wait for worker to finish")
	if !WaitTimeout(&q.wg, 5*time.Second) {
		log.Warn("WaitGroup closed by timeout")
	}
}

func (q *IndexerQueue) Enqueue(task WorkTask) {
	for {
		select {
		case <-time.After(2 * time.Second):
			log.Warn("IndexerQueue.Enqueue timeout")
		case q.jobs <- task:
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

	// don't panic !
	defer func() {
		if rval := recover(); rval != nil {
			log.Errorf("IndexerTask.Do panic: %s", rval)
			debug.PrintStack()
		}
	}()

	err := t.F(t.S)

	fp := strings.Split(runtime.FuncForPC(reflect.ValueOf(t.F).Pointer()).Name(), ".")
	fName := fp[len(fp)-1]
	log.Infof("%s took %s", fName, time.Now().Sub(clock).String())

	if err != nil {
		log.Errorf("IndexerTask.Do %s: %s", fName, err.Error())
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
