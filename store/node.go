package store

import "time"

type Node struct {
	Key   string
	Value []byte

	CreateTime time.Time
	TTL        time.Duration
}

func newNode(key string, value []byte, ttl time.Duration) *Node {
	return &Node{
		Key:        key,
		Value:      value,
		CreateTime: time.Now(),
		TTL:        ttl,
	}
}
