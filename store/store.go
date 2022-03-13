package store

import (
	"time"
)

type Store interface {
	Set(key string, value []byte, expireTime time.Time)
	Get(key string) *Node
	Delete(key string)

	Watch(key string, action Opreation)
}

func NewStore(path string) {

}

type store struct {
	kv      KV
	log     Log
	eventCh chan *Event
	errorCh chan error
}

func (s *store) Set(key string, value []byte, expireTime time.Time) {
	node := newNode(key, value, expireTime)
	s.kv.Set(key, node)

	event := &Event{
		Name: Set,
	}
	s.eventCh <- event
}

func (s *store) Get(key string) *Node {
	value, ok := s.kv.Get(key)
	if !ok {
		return nil
	}

	event := &Event{
		Name: Get,
	}
	s.eventCh <- event

	return value
}

func (s *store) Delete(key string) {
	s.kv.Delete(key)
}

func (s *store) listenEvent() {
	for {
		event := <-s.eventCh

		go func() {
			if err := s.log.Append(event); err != nil {
				s.errorCh <- err
			}
		}()
	}
}

func (s *store) Watch(key string, action Opreation) {

}
