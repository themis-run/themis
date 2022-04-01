package store

import (
	"testing"
	"time"
)

func TestTTLKeyPushNode(t *testing.T) {
	h := newTTLKeyHeap()

	nodes := createTTLKeyNode()
	for _, v := range nodes {
		h.push(v)
	}

	ans := createIndexAns()
	length := h.Len()
	for i := 0; i < length; i++ {
		if nodes[ans[i]] != h.pop() {
			t.Fail()
		}
	}
}

func TestTTLKeyRemoveNode(t *testing.T) {
	h := newTTLKeyHeap()

	nodes := createTTLKeyNode()
	for _, v := range nodes {
		h.push(v)
	}

	h.remove(nodes[2])

	ans := createRemovedAns()
	length := h.Len()
	for i := 0; i < length; i++ {
		if nodes[ans[i]] != h.pop() {
			t.Fail()
		}
	}
}

func createRemovedAns() []int {
	return []int{1, 3, 0, 2}
}

func createIndexAns() []int {
	return []int{1, 3, 0, 2}
}

func createTTLKeyNode() []*Node {
	return []*Node{
		newNode("a", nil, 3*time.Second),
		newNode("b", nil, 1*time.Second),
		newNode("c", nil, 4*time.Second),
		newNode("d", nil, 2*time.Second),
	}
}
