package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"gonet"
	"log"
	"net"

	"time"
)

func main() {

	autoConn, err := gonet.DialAutoReconnectContext(context.Background(), "tcp", "localhost:6606", gonet.WithOnConnected(func(conn gonet.Conn) error {
		line := []byte("0000\n")
		if _, err := conn.Write(line); err != nil {
			return err
		}

		line, _, err := bufio.NewReader(conn).ReadLine()
		_ = line
		if err != nil {
			return errors.New("auth failed")
		}
		return nil
	}))

	if err != nil {
		log.Fatalf("DialAutoReconnect failed, %v", err)
	}

	go intervalWrite(autoConn)
	// go delayClose(autoConn)
	scanner := bufio.NewScanner(autoConn)
	for scanner.Scan() {
		log.Printf("Read line: %s", scanner.Text())
	}
	log.Printf("Exit %v", scanner.Err())
}

func delayClose(conn net.Conn) {
	<-time.After(time.Second * 10)
	conn.Close()
}

func intervalWrite(conn net.Conn) {
	for {
		line := []byte(fmt.Sprintf("%v\n", time.Now()))
		log.Printf("Write line: %s", line)
		_, err := conn.Write(line)
		if err != nil {
			log.Printf("Write failed, %v", err)
			return
		}
		time.Sleep(time.Second * 2)
	}
}
