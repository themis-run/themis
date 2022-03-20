package store

import (
	"container/heap"
)

type ttlKeyHeap struct {
	array    []*Node
	indexMap map[*Node]int
}

func newTTLKeyHeap() *ttlKeyHeap {
	h := &ttlKeyHeap{
		array:    make([]*Node, 0),
		indexMap: make(map[*Node]int),
	}
	heap.Init(h)
	return h
}

func (h *ttlKeyHeap) Len() int {
	return len(h.array)
}

func (h ttlKeyHeap) Less(i, j int) bool {
	return h.array[i].ExpireTime.Before(h.array[j].ExpireTime)
}

func (h ttlKeyHeap) Swap(i, j int) {
	// swap node
	h.array[i], h.array[j] = h.array[j], h.array[i]

	// update map
	h.indexMap[h.array[i]] = i
	h.indexMap[h.array[j]] = j
}

func (h *ttlKeyHeap) Push(x interface{}) {
	n, _ := x.(*Node)
	h.indexMap[n] = len(h.array)
	h.array = append(h.array, n)
}

func (h *ttlKeyHeap) Pop() interface{} {
	old := h.array
	n := len(old)
	x := old[n-1]

	old[n-1] = nil
	h.array = old[0 : n-1]
	delete(h.indexMap, x)
	return x
}

func (h *ttlKeyHeap) top() *Node {
	if h.Len() != 0 {
		return h.array[0]
	}

	return nil
}

func (h *ttlKeyHeap) pop() *Node {
	x := heap.Pop(h)
	n, _ := x.(*Node)
	return n
}

func (h *ttlKeyHeap) push(x interface{}) {
	heap.Push(h, x)
}

func (h *ttlKeyHeap) remove(n *Node) {
	index, ok := h.indexMap[n]
	if ok {
		heap.Remove(h, index)
	}
}
