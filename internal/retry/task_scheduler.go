package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sync/atomic"
)

var ErrSchedulerCanceled = errors.New("task canceled")

type _TaskCallback struct {
	_OnSuccess   func(interface{})
	_OnError     func(error)
	_Retry       func(error)
	_TaskContext func() TaskContext
}

func (s *_TaskCallback) OnSuccess(result interface{}) {
	s._OnSuccess(result)
}

func (s *_TaskCallback) OnError(err error) {
	s._OnError(err)
}

func (s *_TaskCallback) Retry(err error) {
	s._Retry(err)
}

func (s *_TaskCallback) TaskContext() TaskContext {
	return s._TaskContext()
}

type _SignalType int

const (
	_SignalDone _SignalType = iota
	_SignalError
	_SignalRetry
	_SignalTimeout
	_SignalQuit
)

type _Signal struct {
	typ    _SignalType
	result interface{}
	err    error
}

type _Scheduler struct {
	id         uint32
	queue      *_TaskQueue
	signalChan chan *_Signal
	finishChan chan struct{}
}

func _NewScheduler(timeLimit time.Duration) *_Scheduler {
	scheduler := &_Scheduler{
		queue:      _NewTaskQueue(),
		signalChan: make(chan *_Signal, 1),
		finishChan: make(chan struct{}, 1),
	}

	if timeLimit != 0 {
		limitTimer := time.NewTimer(timeLimit)
		go func() {
			select {
			case <-limitTimer.C:
				if !scheduler.queue.IsQueueClosed() {
					scheduler.signalChan <- &_Signal{_SignalError, nil, fmt.Errorf("scheduler deadline[%v] exceeded", timeLimit)}
					scheduler.Finish()
				}
			case <-scheduler.finishChan:
				limitTimer.Stop()
				return
			}
		}()
	} else {
		go func() {
			<-scheduler.finishChan
		}()
	}

	return scheduler
}

func (s *_Scheduler) Invoke(ctx context.Context, task Task, delay time.Duration, timeout time.Duration, evictOldestOne bool, taskContext TaskContext) {
	if s.queue.IsQueueClosed() {
		return
	}

	id := atomic.AddUint32(&s.id, 1)
	taskCtx, cancel := context.WithCancel(ctx)
	s.queue.AddTask(id, cancel, evictOldestOne)
	go s.invokeRoutine(taskCtx, id, task, delay, timeout, taskContext)
}

func (s *_Scheduler) invokeRoutine(ctx context.Context, id uint32, task Task, delay time.Duration, timeout time.Duration, taskContext TaskContext) {
	time.Sleep(delay)

	if timeout != 0 {
		timer := time.NewTimer(timeout)

		go func() {
			select {
			case <-timer.C:
				if s.queue.IsTaskValid(id) {
					s.signalChan <- &_Signal{_SignalTimeout, timeout, nil}
				}
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}()
	}

	task(ctx, &_TaskCallback{
		_OnSuccess: func(result interface{}) {
			if s.queue.IsTaskValid(id) {
				s.signalChan <- &_Signal{_SignalDone, result, nil}
				s.Finish()
			}
		},
		_OnError: func(err error) {
			if s.queue.IsTaskValid(id) {
				s.signalChan <- &_Signal{_SignalError, nil, err}
				s.Finish()
			}
		},
		_Retry: func(err error) {
			if s.queue.IsTaskValid(id) {
				s.signalChan <- &_Signal{_SignalRetry, nil, err}
				s.queue.RemoveTask(id)
			}
		},
		_TaskContext: func() TaskContext {
			return taskContext
		},
	})
}

func (s *_Scheduler) Wait(onDone func(interface{}), onError func(error), onRetry func(error), onTimeout func(time.Duration)) {
	for v := range s.signalChan {
		switch v.typ {
		case _SignalDone:
			onDone(v.result)
		case _SignalError:
			onError(v.err)
		case _SignalRetry:
			onRetry(v.err)
		case _SignalTimeout:
			onTimeout(v.result.(time.Duration))
		case _SignalQuit:
			return
		}
	}
}

func (s *_Scheduler) Finish() {
	if s.queue.IsQueueClosed() {
		return
	}
	s.queue.CloseQueue()
	s.finishChan <- struct{}{}
	s.signalChan <- &_Signal{_SignalQuit, nil, nil}
}

func (s *_Scheduler) Cancel() {
	if s.queue.IsQueueClosed() {
		return
	}
	s.signalChan <- &_Signal{_SignalError, nil, ErrSchedulerCanceled}
	s.Finish()
}
