package sync

import (
	"runtime"
	_sync "sync"

	"github.com/fluffle/goirc/logging"
)

type RWMutex struct {
	m *_sync.RWMutex
}

func (m *RWMutex) init() {
	if m.m == nil {
		m.m = &_sync.RWMutex{}
	}
}

func (m *RWMutex) Lock() {
	m.init()
	_, f, l, _ := runtime.Caller(1)
	logging.Debug("%v: Locked at %v:%v", m, f, l)
	m.m.Lock()
}

func (m *RWMutex) Unlock() {
	m.init()
	_, f, l, _ := runtime.Caller(1)
	logging.Debug("%v: Unlocked at %v:%v", m, f, l)
	m.m.Unlock()
}

func (m *RWMutex) RLock() {
	m.init()
	_, f, l, _ := runtime.Caller(1)
	logging.Debug("%v: Read-locked at %v:%v", m, f, l)
	m.m.RLock()
}

func (m *RWMutex) RUnlock() {
	m.init()
	_, f, l, _ := runtime.Caller(1)
	logging.Debug("%v: Read-unlocked at %v:%v", m, f, l)
	m.m.RUnlock()
}
