package queue

import ()

type Queue struct {
}

func NewPersistentQueue() (*Queue, error) {
	queue := Queue{}
	return &queue, nil
}
