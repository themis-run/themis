package store

import (
	"time"
)

type Store interface {
	Set(key string, value []byte, expireTime time.Time)
	Get(key string) *Node
	Delete(key string)

	Watch(key string, action Action)
}

type store struct {
	kv KV
}

func (s *store) Set(key string, value []byte, expireTime time.Time) {
	node := newNode(key, value, expireTime)
	s.kv.Set(key, node)
}

func (s *store) Get(key string) *Node {
	value, ok := s.kv.Get(key)
	if !ok {
		return nil
	}
	return value
}

func (s *store) Delete(key string) {
	s.kv.Delete(key)
}

func (s *store) Watch(key string, action Action) {

}
