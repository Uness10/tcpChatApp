package main

import (
	"log"
	"net"
)

func main() {
	s := newServer()
	go s.run()

	listener, err := net.Listen("tcp", "0.0.0.0:8888")

	if err != nil {
		log.Fatalf("unable to start server: %s", err.Error())
	}
	defer listener.Close()
	log.Println("Started server on :8888")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Unable to accept connection :%s", err.Error())
			continue
		}
		go s.newClient(conn)

	}
}
