package main

import "sync/atomic"

// ceilingCache provides a thread-safe, lock-free scope ceiling that all
// sidecar handlers read from. After each token renewal, the sidecar updates
// this from the broker's renewal response.
type ceilingCache struct {
	v atomic.Value // holds []string
}

func newCeilingCache(initial []string) *ceilingCache {
	cc := &ceilingCache{}
	cc.v.Store(initial)
	return cc
}

func (c *ceilingCache) get() []string {
	return c.v.Load().([]string)
}

func (c *ceilingCache) set(ceiling []string) {
	c.v.Store(ceiling)
}
