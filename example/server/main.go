package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	l, err := net.Listen("tcp", ":6606")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("Accept connection failed, %v", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	log.Printf("Accept connection %v", conn.RemoteAddr().String())

	// simulate auth
	r := bufio.NewReader(conn)
	line, _, err := r.ReadLine()
	if err != nil {
		log.Printf("Read %v failed, %v", conn.RemoteAddr().String(), err)
		return
	}
	if string(line) != "0000" {
		log.Printf("Auth failed, read %s", line)
		return
	}

	log.Printf("Auth OK!")
	go func() {
		for {
			line, _, err := r.ReadLine()
			if err != nil {
				log.Printf("Read %v failed, %v", conn.RemoteAddr().String(), err)
				return
			}
			log.Printf("Read line: %s", line)
		}
	}()

	for {
		line := []byte(fmt.Sprintf("%v\n", time.Now()))
		log.Printf("Write line: %s", line)

		conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
		_, err := conn.Write(line)
		if err != nil {
			log.Printf("Write %v failed, %v", conn.RemoteAddr().String(), err)
			return
		}
		time.Sleep(time.Second)
	}
}
