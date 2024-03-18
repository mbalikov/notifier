package main

import "sync/atomic"

type AtomicCounter struct {
	value int64
}

func (c *AtomicCounter) Increment() {
	atomic.AddInt64(&c.value, 1)
}

func (c *AtomicCounter) Decrement() {
	atomic.AddInt64(&c.value, -1)
}

func (c *AtomicCounter) Set(val int64) {
	atomic.StoreInt64(&c.value, val)
}

func (c *AtomicCounter) Get() int64 {
	return atomic.LoadInt64(&c.value)
}
