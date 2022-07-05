package promapi

import "sync"

// https://medium.com/@petrlozhkin/kmutex-lock-mutex-by-unique-id-408467659c24
type partitionLocker struct {
	c *sync.Cond
	l sync.Locker
	s map[string]struct{}
}

func newPartitionLocker(l sync.Locker) *partitionLocker {
	return &partitionLocker{c: sync.NewCond(l), l: l, s: make(map[string]struct{})}
}

func (p *partitionLocker) locked(id string) (ok bool) { _, ok = p.s[id]; return }

func (p *partitionLocker) lock(id string) {
	p.l.Lock()
	defer p.l.Unlock()
	for p.locked(id) {
		p.c.Wait()
	}
	p.s[id] = struct{}{}
}

func (p *partitionLocker) unlock(id string) {
	p.l.Lock()
	defer p.l.Unlock()
	delete(p.s, id)
	p.c.Broadcast()
}
