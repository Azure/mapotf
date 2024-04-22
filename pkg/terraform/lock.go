package terraform

import (
	"sync"
)

var lock = &keyLock{
	keyLocks: make(map[string]*sync.Mutex),
}

type keyLock struct {
	glock    sync.Mutex
	keyLocks map[string]*sync.Mutex
}

func (m *keyLock) Lock(key string) {
	m.get(key).Lock()
}

func (m *keyLock) Unlock(key string) {
	m.get(key).Unlock()
}

func (m *keyLock) get(key string) *sync.Mutex {
	m.glock.Lock()
	defer m.glock.Unlock()
	kl, ok := m.keyLocks[key]
	if !ok {
		kl = &sync.Mutex{}
		m.keyLocks[key] = kl
	}
	return kl
}
