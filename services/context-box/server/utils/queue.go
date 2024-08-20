package utils

import "sync"

type SyncQueue struct {
	elements []Identifier
	lock     sync.Mutex
}

func (q *SyncQueue) Enqueue(e Identifier) {
	q.lock.Lock()

	q.elements = append(q.elements, e)

	q.lock.Unlock()
}

func (q *SyncQueue) Dequeue() Identifier {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.elements) == 0 {
		return nil
	}

	dequeuedElement := q.elements[0]
	q.elements = q.elements[1:]

	return dequeuedElement
}

func (q *SyncQueue) Contains(e Identifier) bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	for _, element := range q.elements {
		if element.ID() == e.ID() {
			return true
		}
	}

	return false
}

func (q *SyncQueue) IDs() []string {
	var ids []string

	q.lock.Lock()
	for _, element := range q.elements {
		ids = append(ids, element.ID())
	}
	q.lock.Unlock()

	return ids
}

// CompareElementNameList compares given element-name list with the current element-name list of the queue
func (q *SyncQueue) CompareElementNameList(givenList []string) bool {
	currentList := q.IDs()

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
