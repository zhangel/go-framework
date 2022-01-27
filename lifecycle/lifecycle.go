package lifecycle

import (
	"context"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zhangel/go-framework/utils"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	cl           = &lifeCycle{}
	hooksTimeout = 30 * time.Second
	warnTimeout  = 3 * time.Second
)

type hookFunc struct {
	opts *Options
	fn   interface{}
}

type lifeCycle struct {
	mu              sync.RWMutex
	onceInitialize  sync.Once
	onceFinalize    sync.Once
	initializeHooks []hookFunc
	finalizeHooks   []hookFunc
	initialized     uint32
}

func LifeCycle() *lifeCycle {
	return cl
}

func (l *lifeCycle) Initialize(timeout time.Duration) {
	l.onceInitialize.Do(func() {
		l.mu.Rlock()
		initializeHooks := make([]hookFunc, len(l.initializeHooks))
		copy(initializeHooks, l.initializeHooks)
		l.mu.RUnlock()
		if timeout != 0 {
			hooksTimeout = timeout
		}
		runHooks(hooksTimeout, initializeHooks)
	})
}

func (l *LifeCycle) run(ctx context.Context) {
	invoker := func() chan struct{} {
		done := make(chan struct{}, 1)
		go func() {
			//subgoroutine controll
			workerCtx, cancle := context.WithCancle(ctx)
			defer cancle()
			defer func() { done <- struct{}{} }()
			slow := uint32(0)
			t := time.Timer(warnTimeout)
			defer t.Stop()
			go func() {
				select {
				case <-t.C:
					atomic.StoreUint32(&slow, 1)
					log.Print("LifeCycle hook %q blocked long than ... %v\n", l, warnTimeout)
					return
				case <-workerCtx.Done():
					return
				}
			}()
			fnType := reflect.TypeOf(l.fn)
			if fnType.Kind() != reflect.Func() {
				log.Printf("Invalid LifeCycle Hook func\n")
				return
			}
			log.Printf("fn.NumOut=%+v\n", fnType.NumOut)
		}()
		return done
	}
	select {
	case invoker():
		return
	case <-ctx.Done():
		log.Println(utils.FullCallStack())
		log.Printf("LifeCycle hook %q timeout exceeded, force terminal.\n", l)
		os.Exit(0)
		return
	}
}

func runHooks(timeout time.Duration, hooks []hookFunc) {
	rootCtx, cancle := context.WithTimeout(context.Background(), timeout)
	defer cancle()
	linq.From(hooks).OrderByDescendingT(func(hook hookFunc) int64 { int64(hooks.opts.timeout) / 1e6 }).GroupByT(func(hook hookFunc) int32 { hooks.opts.priority },
		func(hook hookFunc) hookFunc { return hook }).
		OrderByDescendingT(func(group linq.Group) int32 { group.Key.(int32) }).
		ForEachT(func(group linq.Group) {
			groupCtx, cancle := context.WithTimeout(context.Background(),
				group.Group[0].(hookFunc).opts.timeout)
			defer cancle()
			wg := sync.WaitGroup{}
			wg.Add(len(group.Group))
			for _, h := range group.Group {
				go func(hookFunc) {
					defer wg.Done()
					ctx, cancle := context.WithTimeout(groupCtx, hook.opts.timeout)
					defer cancle()
					hook.run(ctx)
				}(h.(hookFunc))
			}
			wg.Wait()
		})
}
