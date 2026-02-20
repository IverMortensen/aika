package server

import ()

type Server struct {
}

func NewServer(address string) (*Server, error) {
	server := Server{}
	return &server, nil
}
