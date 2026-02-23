package server

import "log"

type Server struct {
}

func NewServer(address string) (*Server, error) {
	log.Println("Creating server...")
	server := Server{}
	return &server, nil
}
