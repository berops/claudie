package queue

import (
	"sync"
)

//Data the queue will hold
//In order to evaluate equivalence, the Name must be unique
type ConfigInfo interface {
	GetName() string
}

//Queue uses slice as a data structure to hold elements
//new elements are appended as a last indexes in the slice
//meaning the oldest entries (First in) are at the start of the slice
//Queue is also thread safe and support usage over multiple goroutines
type Queue struct {
	queue []ConfigInfo
	lock  sync.Mutex
}

//Enqueue will add a new element into the end of the queue
func (q *Queue) Enqueue(element ConfigInfo) {
	q.lock.Lock()
	//appends element to last index
	q.queue = append(q.queue, element)
	q.lock.Unlock()
}

//Dequeue will delete oldest element in the queue and return it
//returns nil if queue empty
func (q *Queue) Dequeue() ConfigInfo {
	q.lock.Lock()
	if len(q.queue) == 0 {
		return nil
	}
	//get first element in
	element := q.queue[0]
	//remove the element
	q.queue = q.queue[1:]
	q.lock.Unlock()
	return element
}

//Contains checks if the queue holds the specified element
//returns true if element found, based on the Name, false if no element has the same Name
func (q *Queue) Contains(element ConfigInfo) bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	for _, e := range q.queue {
		if e.GetName() == element.GetName() {
			return true
		}
	}
	return false
}
