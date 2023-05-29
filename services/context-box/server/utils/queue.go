package utils

import "sync"

type QueueElement interface {
	// name of each item must be unique
	GetName() string
}

// Queue uses slice as a data structure to hold elements
// New elements are appended as a last indexes in the slice, meaning the oldest entries (First in) are at the start of the
// slice
// Queue is also thread safe and support usage over multiple go-routines
type Queue struct {
	elements []QueueElement
	lock     sync.Mutex
}

// Enqueue will add a new element into the end of the queue
func (q *Queue) Enqueue(element QueueElement) {
	q.lock.Lock()

	// appends element to last index
	q.elements = append(q.elements, element)

	q.lock.Unlock()
}

// Dequeue will delete oldest element in the queue and return it
// Returns nil if queue is empty
func (q *Queue) Dequeue() QueueElement {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.elements) == 0 {
		return nil
	}

	dequeuedElement := q.elements[0]
	q.elements = q.elements[1:]

	return dequeuedElement
}

// Checks if the queue holds the specified elements
// checking is done by name of the element
func (q *Queue) Contains(targetElement QueueElement) bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	for _, element := range q.elements {
		if element.GetName() == targetElement.GetName() {
			return true
		}
	}

	return false
}

// GetElementNames returns slice of names of the elements in the queue
func (q *Queue) GetElementNames() []string {
	var names []string

	q.lock.Lock()
	for _, element := range q.elements {
		names = append(names, element.GetName())
	}
	q.lock.Unlock()

	return names
}

// CompareElementNameList compares given element-name list with the current element-name list of the queue
func (q *Queue) CompareElementNameList(givenList []string) bool {
	currentList := q.GetElementNames()

	if len(givenList) != len(currentList) {
		return false
	}

	for _, elementName := range currentList {
		if !containsElementName(elementName, givenList) {
			return false
		}
	}

	return true
}

func containsElementName(targetElementName string, givenList []string) bool {
	for _, elementName := range givenList {
		if elementName == targetElementName {
			return true
		}
	}

	return false
}
