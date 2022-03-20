package store

import "time"

// ttlHub
type ttlManager struct {
	ttlKeyHeap *ttlKeyHeap

	// The more KV in the storage, the smaller the value is recommended（Don't equal 0）
	monitorIntervalsTime time.Duration
	eventCh              chan<- *Event
}

func newTTLManager(eventCh chan<- *Event) *ttlManager {
	h := newTTLKeyHeap()

	tm := &ttlManager{
		ttlKeyHeap:           h,
		monitorIntervalsTime: time.Microsecond,
		eventCh:              eventCh,
	}

	go tm.monitorExpireTime()

	return tm
}

func (t *ttlManager) monitorExpireTime() {
	for {
		node := t.ttlKeyHeap.top()
		if node == nil {
			continue
		}

		if time.Now().After(node.ExpireTime) {
			t.expireKey(t.ttlKeyHeap.pop())
		}
		time.Sleep(t.monitorIntervalsTime)
	}
}

func (t *ttlManager) push(node *Node) {
	if node.TTL == 0 {
		return
	}
	t.ttlKeyHeap.push(node)
}

func (t *ttlManager) update(oldNode, newNode *Node) {
	if newNode.TTL == 0 {
		return
	}
	t.ttlKeyHeap.remove(oldNode)
	t.ttlKeyHeap.push(newNode)
}

func (t *ttlManager) remove(node *Node) {
	if node.TTL == 0 {
		return
	}
	t.ttlKeyHeap.remove(node)
}

func (t *ttlManager) expireKey(node *Node) {
	node.Expired = true

	event := &Event{
		Name: Expire,
		Node: node,
	}
	t.eventCh <- event
}
