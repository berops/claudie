package worker

import "log"

func ErrorLogger(err error) {
	log.Println(err)
}
