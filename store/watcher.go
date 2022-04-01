package store

import (
	"container/list"
	"sync"
	"sync/atomic"
)

type Opreation = string

const (
	All    = "all"
	Set    = "set"
	Get    = "get"
	Delete = "delete"
	Write  = "write" // Set, Delete, Expire
	Expire = "expire"
)

type Event struct {
	Name    Opreation
	OldNode *Node
	Node    *Node
}

var (
	MaxWatcherNum       = 30
	MaxWatcherNotifyNum = 10
)

type watcherHub struct {
	sync.Mutex
	watcherMap   map[string]*list.List
	watcherCount int64
}

func newWatcherHub() *watcherHub {
	return &watcherHub{
		watcherMap: make(map[string]*list.List),
	}
}

func (wh *watcherHub) newWatcher(key string, op Opreation, isStream bool) Watcher {
	wh.Lock()
	defer wh.Unlock()

	if _, ok := wh.watcherMap[key]; !ok {
		wh.watcherMap[key] = list.New()
	}

	w := newWatcher(wh, op, isStream)
	elem := wh.watcherMap[key].PushBack(w)

	w.remove = func() {
		if w.removed {
			return
		}
		w.removed = true

		l, ok := wh.watcherMap[key]
		if !ok {
			return
		}

		l.Remove(elem)
		atomic.AddInt64(&wh.watcherCount, -1)

		if l.Len() == 0 {
			delete(wh.watcherMap, key)
		}
	}

	atomic.AddInt64(&wh.watcherCount, 1)
	return w
}

func (wh *watcherHub) liveWatcherCount() int64 {
	return wh.watcherCount
}

func (wh *watcherHub) notify(event *Event) {
	wh.Lock()
	defer wh.Unlock()

	l, ok := wh.watcherMap[event.Node.Key]
	if !ok {
		return
	}

	curr := l.Front()
	for curr != nil {
		next := curr.Next()
		w := curr.Value.(*watcher)

		if wh.isNotify(w.Opreation(), event.Name) {
			w.notify(event)
		}

		curr = next
	}
}

// isSubOpreate 判断是否需要唤醒.
func (wh *watcherHub) isNotify(op1, op2 Opreation) bool {
	switch op1 {
	case All:
		return true
	case Write:
		return op2 != Get
	}
	return op1 == op2
}

type Watcher interface {
	Opreation() Opreation
	EventChan() <-chan *Event
	Remove()
}

type watcher struct {
	op         Opreation
	eventChan  chan *Event
	watcherHub *watcherHub
	isStream   bool
	remove     func()
	removed    bool
}

func newWatcher(wa *watcherHub, op string, isStream bool) *watcher {
	return &watcher{
		watcherHub: wa,
		op:         op,
		eventChan:  make(chan *Event, MaxWatcherNotifyNum),
		isStream:   isStream,
	}
}

func (w *watcher) Opreation() Opreation {
	return w.op
}

func (w *watcher) EventChan() <-chan *Event {
	return w.eventChan
}

func (w *watcher) notify(event *Event) {
	w.eventChan <- event
}

func (w *watcher) Remove() {
	w.watcherHub.Lock()
	defer w.watcherHub.Unlock()

	close(w.eventChan)
	w.remove()
}
