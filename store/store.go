package store

import (
	"time"
)

type Store interface {
	Set(key string, value []byte, ttl time.Duration)
	Get(key string) *Node
	Delete(key string)

	Watch(key string, action Opreation)
}

func NewStore(path string, size uint) (Store, error) {
	l, err := NewLog(path)
	if err != nil {
		return nil, err
	}

	return &store{
		kv:         newKVStore(uintptr(size)),
		log:        l,
		watcherHub: *newWatcherHub(),
		eventCh:    make(chan *Event, 100),
		errorCh:    make(chan error, 5),
	}, nil
}

type store struct {
	kv         KV
	log        Log
	watcherHub watcherHub
	eventCh    chan *Event
	errorCh    chan error
}

func (s *store) Set(key string, value []byte, ttl time.Duration) {
	node := newNode(key, value, ttl)
	s.eventCh <- s.kv.Set(key, node)
}

func (s *store) Get(key string) *Node {
	event, ok := s.kv.Get(key)
	if !ok {
		return nil
	}

	s.eventCh <- event

	return event.Node
}

func (s *store) Delete(key string) {
	s.eventCh <- s.kv.Delete(key)
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

func (s *store) Watch(key string, action Opreation) {

}
