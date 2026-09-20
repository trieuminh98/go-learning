package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	address      = ":8080"
	readTimeout  = 30 * time.Second
	writeTimeout = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := serve(ctx, address); err != nil {
		log.Fatal(err)
	}
}

func serve(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("close listener: %v", err)
		}
	}()

	log.Printf("TCP server listening on %s", listener.Addr())

	serverDone := make(chan struct{})
	defer close(serverDone)
	go func() {
		select {
		case <-ctx.Done():
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				log.Printf("close listener during shutdown: %v", err)
			}
		case <-serverDone:
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept connection: %w", err)
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("close connection %s: %v", conn.RemoteAddr(), err)
		}
	}()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("set read deadline for %s: %v", conn.RemoteAddr(), err)
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("read from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		response, closeConnection := processCommand(line)
		if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			log.Printf("set write deadline for %s: %v", conn.RemoteAddr(), err)
			return
		}
		if _, err := writer.WriteString(response); err != nil {
			log.Printf("write to %s: %v", conn.RemoteAddr(), err)
			return
		}
		if err := writer.Flush(); err != nil {
			log.Printf("flush to %s: %v", conn.RemoteAddr(), err)
			return
		}
		if closeConnection {
			return
		}
	}
}

func processCommand(line string) (response string, closeConnection bool) {
	command := strings.TrimRight(line, "\r\n")

	switch {
	case command == "PING":
		return "PONG\n", false
	case strings.HasPrefix(command, "ECHO "):
		return strings.TrimPrefix(command, "ECHO ") + "\n", false
	case command == "QUIT":
		return "BYE\n", true
	default:
		return "ERR unknown command\n", false
	}
}
