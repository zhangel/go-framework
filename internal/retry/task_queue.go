package retry

import (
	"context"
	"sync"
)

type _Task struct {
	id     uint32
	cancel context.CancelFunc
	valid  bool
}

type _TaskQueue struct {
	mutex     sync.RWMutex
	taskQueue []*_Task
	closed    bool
}

func _NewTaskQueue() *_TaskQueue {
	return &_TaskQueue{
		taskQueue: []*_Task{},
		closed:    false,
	}
}

func (s *_TaskQueue) AddTask(id uint32, cancel context.CancelFunc, evictOldestOne bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if evictOldestOne {
		for idx := range s.taskQueue {
			if s.taskQueue[idx].valid {
				s.taskQueue[idx].valid = false
				s.taskQueue[idx].cancel()
				break
			}
		}
	}

	s.taskQueue = append(s.taskQueue, &_Task{id, cancel, true})
}

func (s *_TaskQueue) RemoveTask(id uint32) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for idx := range s.taskQueue {
		if s.taskQueue[idx].id == id {
			s.taskQueue[idx].valid = false
			s.taskQueue[idx].cancel()
		}
	}
}

func (s *_TaskQueue) CloseQueue() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for idx := range s.taskQueue {
		if s.taskQueue[idx].valid {
			s.taskQueue[idx].valid = false
			s.taskQueue[idx].cancel()
		}
	}

	s.taskQueue = []*_Task{}
	s.closed = true
}

func (s *_TaskQueue) IsTaskValid(id uint32) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.closed {
		return false
	}

	for idx := range s.taskQueue {
		if s.taskQueue[idx].id == id {
			return s.taskQueue[idx].valid
		}
	}

	return false
}

func (s *_TaskQueue) TaskCount() uint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.closed {
		return 0
	}

	count := uint(0)
	for idx := range s.taskQueue {
		if s.taskQueue[idx].valid {
			count++
		}
	}

	return count
}

func (s *_TaskQueue) IsQueueClosed() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.closed
}
