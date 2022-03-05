package retry

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrDuplicateId  = errors.New("duplicate invoke uniqueId found")
	ErrRetryTimeout = errors.New("retry timeout exceeded")
)

type TaskCallback interface {
	OnSuccess(interface{})
	OnError(error)
	Retry(error)
	TaskContext() TaskContext
}

// Task定义了需要通过RetryController进行重试的函数原型
type Task func(ctx context.Context, cb TaskCallback)

type TaskContext struct {
	RetryNum       uint          // 已经重试的次数
	ConcurrencyNum uint          // 当前的并发重试数
	StartTime      time.Time     // 任务开始时间
	TotalLimit     time.Duration // 任务最多可以使用的时间
	LastError      error         // 最后一次的错误
}

type TimePolicy interface {
	Calc(ctx context.Context, taskContext *TaskContext) time.Duration
}

type TimePolicyFunc func(ctx context.Context, taskContext *TaskContext) time.Duration

func (s TimePolicyFunc) Calc(ctx context.Context, taskContext *TaskContext) time.Duration {
	return s(ctx, taskContext)
}

type BooleanPolicy interface {
	Calc(ctx context.Context, taskContext *TaskContext) bool
}

type BooleanPolicyFunc func(ctx context.Context, taskContext *TaskContext) bool

func (s BooleanPolicyFunc) Calc(ctx context.Context, taskContext *TaskContext) bool {
	return s(ctx, taskContext)
}

type Options struct {
	// 总超时时间
	timeLimit time.Duration

	// 超时时间策略
	timeoutPolicy TimePolicy

	// 重试间隔策略
	intervalPolicy TimePolicy

	// 是否继续重试策略
	retryPolicy BooleanPolicy

	// 是否并发重试策略
	concurrencyPolicy BooleanPolicy

	// 重试整体达到超时的错误值
	timeoutErrGenerator func(timeout time.Duration) error

	// 取消重试时的错误值
	cancelError error
}

type RetryController interface {
	Invoke(ctx context.Context, task Task, opt ...func(*InvokeOptions)) (interface{}, error)
	HasTask(id string) bool
	CancelTask(id string)
}

type _RetryController struct {
	opts *Options

	mu      sync.RWMutex
	taskMap map[string]*_Scheduler
}

func NewRetryController(optFuncs ...func(*Options)) RetryController {
	rc := &_RetryController{
		opts: &Options{
			timeoutErrGenerator: func(timeout time.Duration) error { return ErrRetryTimeout },
			cancelError:         ErrSchedulerCanceled,
		},
		taskMap: make(map[string]*_Scheduler),
	}

	for _, opt := range optFuncs {
		opt(rc.opts)
	}

	if rc.opts.timeoutPolicy == nil {
		panic("No timeout policy specified.")
	}

	if rc.opts.intervalPolicy == nil {
		panic("No interval policy specified.")
	}

	if rc.opts.retryPolicy == nil {
		panic("No retry policy specified.")
	}

	if rc.opts.concurrencyPolicy == nil {
		panic("No concurrency policy specified.")
	}

	return rc
}

func WithTimeLimit(timeLimit time.Duration) func(*Options) {
	return func(opts *Options) {
		opts.timeLimit = timeLimit
	}
}

func WithTimeoutPolicy(timeoutPolicy TimePolicy) func(*Options) {
	return func(opts *Options) {
		opts.timeoutPolicy = timeoutPolicy
	}
}

func WithRetryIntervalPolicy(retryIntervalPolicy TimePolicy) func(*Options) {
	return func(opts *Options) {
		opts.intervalPolicy = retryIntervalPolicy
	}
}

func WithRetryPolicy(retryPolicy BooleanPolicy) func(*Options) {
	return func(opts *Options) {
		opts.retryPolicy = retryPolicy
	}
}

func WithConcurrencyPolicy(concurrencyPolicy BooleanPolicy) func(*Options) {
	return func(opts *Options) {
		opts.concurrencyPolicy = concurrencyPolicy
	}
}

func WithTimeoutError(errGenerator func(timeout time.Duration) error) func(*Options) {
	return func(opts *Options) {
		opts.timeoutErrGenerator = errGenerator
	}
}

func WithCancelError(err error) func(*Options) {
	return func(opts *Options) {
		opts.cancelError = err
	}
}

type InvokeOptions struct {
	taskId string
}

func InvokeWithTaskId(id string) func(*InvokeOptions) {
	return func(opts *InvokeOptions) {
		opts.taskId = id
	}
}

func (s *_RetryController) Invoke(ctx context.Context, task Task, opt ...func(*InvokeOptions)) (interface{}, error) {
	var result interface{}
	var err error

	opts := &InvokeOptions{}
	for _, o := range opt {
		o(opts)
	}

	if opts.taskId != "" {
		s.mu.RLock()
		if _, ok := s.taskMap[opts.taskId]; ok {
			s.mu.RUnlock()
			return nil, ErrDuplicateId
		}
		s.mu.RUnlock()

		s.mu.Lock()
		if _, ok := s.taskMap[opts.taskId]; ok {
			s.mu.Unlock()
			return nil, ErrDuplicateId
		}

		defer func() {
			s.mu.Lock()
			defer s.mu.Unlock()

			delete(s.taskMap, opts.taskId)
		}()
	}

	scheduler := _NewScheduler(s.opts.timeLimit)
	if opts.taskId != "" {
		s.taskMap[opts.taskId] = scheduler
		s.mu.Unlock()
	}

	taskContext := TaskContext{
		RetryNum:       0,
		ConcurrencyNum: 0,
		StartTime:      time.Now(),
		TotalLimit:     s.opts.timeLimit,
		LastError:      nil,
	}

	retry := func(scheduler *_Scheduler, e error) {
		taskContext.LastError = e
		taskContext.RetryNum++
		taskContext.ConcurrencyNum = scheduler.queue.TaskCount()
		if s.opts.retryPolicy.Calc(ctx, &taskContext) {
			scheduler.Invoke(ctx, task, s.opts.intervalPolicy.Calc(ctx, &taskContext), s.opts.timeoutPolicy.Calc(ctx, &taskContext), !s.opts.concurrencyPolicy.Calc(ctx, &taskContext), taskContext)
		} else {
			err = e
			scheduler.Finish()
		}
	}

	scheduler.Invoke(ctx, task, s.opts.intervalPolicy.Calc(ctx, &taskContext), s.opts.timeoutPolicy.Calc(ctx, &taskContext), false, taskContext)
	scheduler.Wait(func(v interface{}) { result = v }, func(e error) {
		if e == ErrSchedulerCanceled {
			err = s.opts.cancelError
		} else {
			err = e
		}
	}, func(e error) { retry(scheduler, e) }, func(timeout time.Duration) {
		retry(scheduler, s.opts.timeoutErrGenerator(timeout))
	})
	return result, err
}

func (s *_RetryController) HasTask(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.taskMap[id]
	return ok
}

func (s *_RetryController) CancelTask(id string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scheduler, ok := s.taskMap[id]
	if ok {
		scheduler.Cancel()
	}
}
