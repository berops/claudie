package worker

import "log"

// ErrorLogger function defines a callback for handling errors
func ErrorLogger(err error) {
	log.Println(err)
}
