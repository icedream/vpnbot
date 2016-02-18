package util

import (
	"sync"

	"github.com/fluffle/goirc/client"
)

type ModeChange struct {
	Mode string
	Args []string
}

type ModeChangeBuffer struct {
	c      *client.Conn
	modes  uint64
	buf    []ModeChange
	mutex  *sync.Mutex
	target string
}

func NewModeChangeBuffer(c *client.Conn, maxModes uint64) *ModeChangeBuffer {
	return &ModeChangeBuffer{
		c:     c,
		buf:   []ModeChange{},
		modes: maxModes,
		mutex: &sync.Mutex{},
	}
}

func (m *ModeChangeBuffer) flushNoSync() {
	// Get a copy of the buffer as early as possible...
	oldBuf := m.buf
	if len(oldBuf) == 0 {
		return
	}

	// ...and then empty the buffer immediately.
	m.buf = []ModeChange{}

	// Generate strings to pass to Mode()
	modeArgs := []string{""}
	for _, change := range oldBuf {
		modeArgs[0] += change.Mode
		modeArgs = append(modeArgs, change.Args...)
	}

	// Pass over to Mode()
	m.c.Mode(m.target, modeArgs...)
}

func (m *ModeChangeBuffer) Flush() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.flushNoSync()
}

func (m *ModeChangeBuffer) Mode(target string, mode string, arg ...string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Flush if different target
	if m.target == "" {
		m.target = target
	} else if m.target != target {
		m.flushNoSync()
		m.target = target
	}

	// Append to buffer
	m.buf = append(m.buf, ModeChange{Mode: mode, Args: arg})

	// Flush if maximum reached
	if uint64(len(m.buf)) == m.modes {
		m.flushNoSync()
	}
}
