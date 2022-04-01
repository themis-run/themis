package store

import (
	"strings"
	"time"
)

type Store interface {
	Set(key string, value []byte, ttl time.Duration) *Event
	Get(key string) *Event
	Delete(key string) *Event

	Watch(key string, action Opreation, isStream bool) Watcher

	ListAllNode() []*Node
	ListNodeByPrefix(prefix string) []*Node
}

func New(path string, size uint) (Store, error) {
	l, err := NewLog(path)
	if err != nil {
		return nil, err
	}

	eventCh := make(chan *Event, 100)

	s := &store{
		kv:         newKVStore(uintptr(size)),
		log:        l,
		watcherHub: newWatcherHub(),
		ttlManager: newTTLManager(eventCh),
		eventCh:    eventCh,
		errorCh:    make(chan error, 5),
	}

	go s.listenEvent()
	return s, nil
}

type store struct {
	kv         KV
	log        Log
	watcherHub *watcherHub
	ttlManager *ttlManager
	eventCh    chan *Event
	errorCh    chan error
}

func (s *store) Set(key string, value []byte, ttl time.Duration) *Event {
	node := newNode(key, value, ttl)
	event := s.kv.Set(key, node)

	if event.OldNode != nil {
		s.ttlManager.update(event.OldNode, event.Node)
	} else {
		s.ttlManager.push(event.Node)
	}

	s.eventCh <- event
	return event
}

func (s *store) Get(key string) *Event {
	event, ok := s.kv.Get(key)
	if !ok {
		return nil
	}

	s.eventCh <- event

	return event
}

func (s *store) Delete(key string) *Event {
	event := s.kv.Delete(key)

	s.ttlManager.remove(event.Node)
	s.eventCh <- event
	return event
}

func (s *store) listenEvent() {
	for {
		event := <-s.eventCh

		go func() {
			if err := s.log.Append(event); err != nil {
				s.errorCh <- err
			}
		}()

		go func() {
			s.watcherHub.notify(event)
		}()
	}
}

func (s *store) Watch(key string, action Opreation, isStream bool) Watcher {
	return s.watcherHub.newWatcher(key, action, isStream)
}

func (s *store) ListAllNode() []*Node {
	return s.ListNodeByPrefix("")
}

func (s *store) ListNodeByPrefix(prefix string) []*Node {
	itr := s.kv.ListNodeByPreKey(prefix)
	nodeList := make([]*Node, 0)

	for v := range itr {
		if strings.HasPrefix(v.Key, prefix) {
			nodeList = append(nodeList, &v)
		}
	}

	return nodeList
}
