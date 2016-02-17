package mode

/*
This code has been mostly copied from:
  https://github.com/fluffle/goirc/blob/8be75dd9d4b2b2be3519a9dc9612ead627b7a721/client/dispatch.go
*/

import (
	"strings"
	"sync"

	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"
)

// Handlers are triggered on incoming mode changes from the server. The handler
// name corresponds to the mode change that has been done, with the first
// character being either "+" (add) or "-" (remove) and the second being the
// mode character. The handler name can also be "*" to handle all mode changes.
//
// Foreground handlers have a guarantee of protocol consistency: all the
// handlers for one event will have finished before the handlers for the
// next start processing. They are run in parallel but block the event
// loop, so care should be taken to ensure these handlers are quick :-)
//
// Background handlers are run in parallel and do not block the event loop.
// This is useful for things that may need to do significant work.
type ModeChangeHandler interface {
	Handle(*ModeChangeEvent)
}

// HandlerFunc allows a bare function with this signature to implement the
// Handler interface. It is used by Plugin.HandleFunc.
type ModeChangeHandlerFunc func(*ModeChangeEvent)

func (hf ModeChangeHandlerFunc) Handle(e *ModeChangeEvent) {
	hf(e)
}

// Handlers are organised using a map of linked-lists, with each map
// key representing an IRC verb or numeric, and the linked list values
// being handlers that are executed in parallel when a Line from the
// server with that verb or numeric arrives.
type hSet struct {
	set map[string]*hList
	sync.RWMutex
}

type hList struct {
	start, end *hNode
}

// Storing the forward and backward links in the node allows O(1) removal.
// This probably isn't strictly necessary but I think it's kinda nice.
type hNode struct {
	next, prev *hNode
	set        *hSet
	event      string
	handler    ModeChangeHandler
}

// A hNode implements both Handler ...
func (hn *hNode) Handle(e *ModeChangeEvent) {
	hn.handler.Handle(e)
}

// ... and Remover.
func (hn *hNode) Remove() {
	hn.set.remove(hn)
}

func handlerSet() *hSet {
	return &hSet{set: make(map[string]*hList)}
}

// When a new Handler is added for an event, it is wrapped in a hNode and
// returned as a Remover so the caller can remove it at a later time.
func (hs *hSet) add(ev string, h ModeChangeHandler) client.Remover {
	hs.Lock()
	defer hs.Unlock()
	ev = strings.ToLower(ev)
	l, ok := hs.set[ev]
	if !ok {
		l = &hList{}
	}
	hn := &hNode{
		set:     hs,
		event:   ev,
		handler: h,
	}
	if !ok {
		l.start = hn
	} else {
		hn.prev = l.end
		l.end.next = hn
	}
	l.end = hn
	hs.set[ev] = l
	return hn
}

func (hs *hSet) remove(hn *hNode) {
	hs.Lock()
	defer hs.Unlock()
	l, ok := hs.set[hn.event]
	if !ok {
		logging.Error("Removing node for unknown event '%s'", hn.event)
		return
	}
	if hn.next == nil {
		l.end = hn.prev
	} else {
		hn.next.prev = hn.prev
	}
	if hn.prev == nil {
		l.start = hn.next
	} else {
		hn.prev.next = hn.next
	}
	hn.next = nil
	hn.prev = nil
	hn.set = nil
	if l.start == nil || l.end == nil {
		delete(hs.set, hn.event)
	}
}

func (hs *hSet) dispatch(name string, e *ModeChangeEvent) {
	hs.RLock()
	defer hs.RUnlock()
	ev := strings.ToLower(name)
	list, ok := hs.set[ev]
	if !ok {
		return
	}
	wg := &sync.WaitGroup{}
	for hn := list.start; hn != nil; hn = hn.next {
		wg.Add(1)
		go func(hn *hNode) {
			hn.Handle(e)
			wg.Done()
		}(hn)
	}
	wg.Wait()
}

// Handle adds the provided handler to the foreground set for the named event.
// It will return a Remover that allows that handler to be removed again.
func (plugin *Plugin) Handle(name string, h ModeChangeHandler) client.Remover {
	return plugin.fgHandlers.add(name, h)
}

// HandleBG adds the provided handler to the background set for the named
// event. It may go away in the future.
// It will return a Remover that allows that handler to be removed again.
func (plugin *Plugin) HandleBG(name string, h ModeChangeHandler) client.Remover {
	return plugin.bgHandlers.add(name, h)
}

func (plugin *Plugin) handle(name string, h ModeChangeHandler) client.Remover {
	return plugin.intHandlers.add(name, h)
}

// HandleFunc adds the provided function as a handler in the foreground set
// for the named event.
// It will return a Remover that allows that handler to be removed again.
func (plugin *Plugin) HandleFunc(name string, hf ModeChangeHandlerFunc) client.Remover {
	return plugin.Handle(name, hf)
}

func (plugin *Plugin) dispatch(name string, e *ModeChangeEvent) {
	// We run the internal handlers first, including all state tracking ones.
	// This ensures that user-supplied handlers that use the tracker have a
	// consistent view of the connection state in handlers that mutate it.
	plugin.intHandlers.dispatch(name, e)
	go plugin.bgHandlers.dispatch(name, e)
	plugin.fgHandlers.dispatch(name, e)
}
