package store

import "time"

type Node struct {
	Key   string
	Value []byte

	ExpireTime time.Time
}

func newNode(key string, value []byte, expireTime time.Time) *Node {
	return &Node{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
	}
}
