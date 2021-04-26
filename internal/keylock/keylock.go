package keylock

import "sync"

// https://medium.com/@petrlozhkin/kmutex-lock-mutex-by-unique-id-408467659c24
type PartitionLocker struct {
	c *sync.Cond
	l sync.Locker
	s map[string]struct{}
}

func NewPartitionLocker(l sync.Locker) *PartitionLocker {
	return &PartitionLocker{c: sync.NewCond(l), l: l, s: make(map[string]struct{})}
}

func (p *PartitionLocker) locked(id string) (ok bool) { _, ok = p.s[id]; return }

func (p *PartitionLocker) Lock(id string) {
	p.l.Lock()
	defer p.l.Unlock()
	for p.locked(id) {
		p.c.Wait()
	}
	p.s[id] = struct{}{}
}

func (p *PartitionLocker) Unlock(id string) {
	p.l.Lock()
	defer p.l.Unlock()
	delete(p.s, id)
	p.c.Broadcast()
}
