package store

import "time"

type Node struct {
	Key   string
	Value []byte

	TTL time.Duration
}

func newNode(key string, value []byte, ttl time.Duration) *Node {
	return &Node{
		Key:   key,
		Value: value,
		TTL:   ttl,
	}
}
