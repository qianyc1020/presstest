package util

import "sync"

//Semaphore 信号量互斥访问控制
type Semaphore struct {
	w        *sync.Mutex
	l        *sync.Mutex
	c        *sync.Cond
	avail    int64
	initsize int64
}

//NewSemaphore 初始化initsize个并发访问资源
func NewSemaphore(initsize int64) *Semaphore {
	s := &Semaphore{initsize: initsize, avail: initsize, l: &sync.Mutex{}, w: &sync.Mutex{}}
	s.c = sync.NewCond(s.l)
	return s
}

//Enter 进入访问资源
func (s *Semaphore) Enter() {
wait:
	s.wait()
	s.w.Lock()
	if s.avail > 0 {
		s.avail--
		s.w.Unlock()
	} else {
		s.w.Unlock()
		goto wait
	}
}

//Leave 离开释放资源
func (s *Semaphore) Leave() {
	s.w.Lock()
	if s.avail < s.initsize {
		s.avail++
		s.c.Signal()
	}
	s.w.Unlock()
}

//wait 等待资源
func (s *Semaphore) wait() {
	s.l.Lock()
	if s.avail == 0 {
		s.c.Wait()
	}
	s.l.Unlock()
}
