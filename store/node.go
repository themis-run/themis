package store

import "time"

type Node struct {
	Key   string
	Value []byte

	Expired    bool
	CreateTime time.Time
	ExpireTime time.Time
	TTL        time.Duration
}

func newNode(key string, value []byte, ttl time.Duration) *Node {
	now := time.Now()
	n := &Node{
		Key:        key,
		Value:      value,
		Expired:    false,
		CreateTime: now,
		ExpireTime: now.Add(ttl),
		TTL:        ttl,
	}

	n.ExpireTime = now.Add(ttl)

	return n
}

func (n *Node) IsExpire() bool {
	return time.Now().After(n.ExpireTime)
}

func (n *Node) UpdateTTL(ttl time.Duration) {
	n.TTL = ttl
	n.ExpireTime = time.Now().Add(ttl)
}
