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
	elements         []QueueElement
	threadSafetyLock sync.Mutex
}

// Enqueue will add a new element into the end of the queue
func (q *Queue) Enqueue(element QueueElement) {
	q.threadSafetyLock.Lock()

	// appends element to last index
	q.elements = append(q.elements, element)

	q.threadSafetyLock.Unlock()
}

// Dequeue will delete oldest element in the queue and return it
// Returns nil if queue is empty
func (q *Queue) Dequeue() QueueElement {
	q.threadSafetyLock.Lock()
	defer q.threadSafetyLock.Unlock()

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
	q.threadSafetyLock.Lock()
	defer q.threadSafetyLock.Unlock()

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

	q.threadSafetyLock.Lock()
	for _, element := range q.elements {
		names = append(names, element.GetName())
	}
	defer q.threadSafetyLock.Unlock()

	return names
}

// CompareElementNameList compares given element-name list with the current element-name list of the queue
func (q *Queue) CompareElementNameList(givenList []string) bool {
	currentList := q.GetElementNames()

	if len(givenList) != len(currentList) {
		return false
	}

	for _, elementName := range currentList {
		if !doesListContainElementName(elementName, givenList) {
			return false
		}
	}

	return true
}

func doesListContainElementName(targetElementName string, givenList []string) bool {
	for _, elementName := range givenList {
		if elementName == targetElementName {
			return true
		}
	}

	return false
}
