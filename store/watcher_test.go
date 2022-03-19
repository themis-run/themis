package store

import (
	"sync"
	"testing"
)

func TestNewWatcher(t *testing.T) {
	w := newWatcherHub()
	kos := createKeyOpreationData()

	for _, v := range kos {
		w.newWatcher(v.key, v.op, false)
	}
}

func TestNotify(t *testing.T) {
	w := newWatcherHub()
	kos := createKeyOpreationData()
	watchers := make([]Watcher, 0)
	ans := make([]*Event, 0)

	for _, v := range kos {
		watchers = append(watchers, w.newWatcher(v.key, v.op, false))
	}

	events := createEventData()
	for _, event := range events {
		for _, v := range kos {
			if event.Node.Key == v.key && w.isNotify(v.op, event.Name) {
				ans = append(ans, event)
			}
		}

		w.notify(event)
	}

	successCount := 0
	var wg sync.WaitGroup
	wg.Add(len(watchers))
	for _, watcher := range watchers {
		go func(watcher Watcher) {
			for {
				var event *Event
				select {
				case event = <-watcher.EventChan():
				default:
					wg.Done()
					return
				}

				success := false

				for _, e := range ans {
					if event == e {
						success = true
						successCount++
						break
					}
				}

				if !success {
					t.Fail()
				}
			}
		}(watcher)
	}

	wg.Wait()

	if successCount != len(ans) {
		t.Fail()
	}
}

func TestRemove(t *testing.T) {
	w := newWatcherHub()
	kos := createKeyOpreationData()
	watchers := make([]Watcher, 0)

	for _, v := range kos {
		watchers = append(watchers, w.newWatcher(v.key, v.op, false))
	}

	for _, watcher := range watchers {
		watcher.Remove()
	}

	if w.liveWatcherCount() != 0 {
		t.Fail()
	}
}

func createEventData() []*Event {
	keys := []string{"a", "b", "c"}
	ret := make([]*Event, 0)
	for _, key := range keys {
		ret = append(ret, createEventDataByKey(key)...)
	}
	return ret
}

func createEventDataByKey(key string) []*Event {
	var ops []Opreation = []Opreation{Set, Get, Expire, Delete}
	ret := make([]*Event, 0)
	for _, op := range ops {
		e := &Event{
			Name: op,
			Node: newEventNode(key),
		}
		ret = append(ret, e)
	}
	return ret
}

func newEventNode(key string) *Node {
	return newNode(key, nil, 1)
}

type keyAndOpreation struct {
	key string
	op  Opreation
}

func createKeyOpreationData() []keyAndOpreation {
	return []keyAndOpreation{
		{
			key: "a",
			op:  Get,
		},
		{
			key: "a",
			op:  Set,
		},
		{
			key: "b",
			op:  All,
		},
		{
			key: "c",
			op:  Write,
		},
	}
}
