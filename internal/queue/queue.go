package queue

import "log"

type Queue struct {
}

func NewPersistentQueue() (*Queue, error) {
	log.Println("Creating queue...")
	queue := Queue{}
	return &queue, nil
}
