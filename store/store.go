package store

import "time"

type Store interface {
	Set(key string, value []byte, expireTime time.Time)
	Get(key string) Node
	Delete(key string)

	Watch(key string, action Action)
}

type Node struct {
	Key   string
	Value []byte

	ExpireTime time.Time
}
