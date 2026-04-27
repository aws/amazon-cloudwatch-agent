// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package priorityqueue

import "cmp"

// QueueItem represents an item in the priority queue.
type QueueItem[V any, P cmp.Ordered] struct {
	Value    V
	Priority P
	Index    int
}

// PriorityQueue implements heap.Interface and holds QueueItems.
type PriorityQueue[V any, P cmp.Ordered] []*QueueItem[V, P]

func (pq PriorityQueue[V, P]) Len() int { return len(pq) }

func (pq PriorityQueue[V, P]) Less(i, j int) bool {
	// We want Pop to give us the highest priority, so we use greater than here.
	return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue[V, P]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue[V, P]) Push(x any) {
	n := len(*pq)
	item := x.(*QueueItem[V, P])
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue[V, P]) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	*pq = old[0 : n-1]
	return item
}
