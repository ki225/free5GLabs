package lab0

// This code runs in user space because it uses Go's net package.
import (
	"errors"
	"io"
	"log"
	"net"
	"strconv"
)

type listenerInterface func(string, int, handlerInterface)

type handlerInterface func(conn net.Conn)

const maxConcurrentConnections = 100

func TCPListener(host string, port int, handler handlerInterface) {
	address := net.JoinHostPort(host, strconv.Itoa(port))

	// socket(), bind(), listen(): net.Listen creates a TCP socket, binds it to
	// the address, and starts listening for incoming connections.
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Printf("listen on %s failed: %v", address, err)
		return
	}

	// defer postpones listener.Close() until TCPListener is about to return.
	// This makes sure the server socket is closed before the function ends.
	defer listener.Close()

	// sem is a buffered channel used as a semaphore. It limits how many client
	// connections can run at the same time.
	// The channel can hold maxConcurrentConnections tokens.
	sem := make(chan struct{}, maxConcurrentConnections)

	for {
		// accept(): wait for the next client connection.
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Printf("listener closed: %v", err)
				return
			}
			log.Printf("accept failed: %v", err)
			// A single Accept error does not always mean the listener is unusable.
			// Keep the server running so it can still accept later clients.
			continue
		}

		// struct{}{} is an empty struct value. Sending it into sem takes one
		// connection slot.
		sem <- struct{}{}
		go func() {
			// When the handler finishes, receive one item from sem to release
			// the connection slot for the next accepted client.
			defer func() {
				// Receiving from sem removes one token, so the slot is released.
				<-sem
			}()

			// This goroutine handles the client connection returned by Accept().
			// When handler returns, this goroutine ends and the deferred release runs.
			handler(conn)
		}()
	}
}

func TCPHandler(conn net.Conn) {
	// defer postpones conn.Close() until TCPHandler is about to return.
	// This makes sure each client connection is closed after handling.
	defer conn.Close()

	// buf stores bytes received from the TCP connection.
	buf := make([]byte, 1024)
	for {
		// recv(): read bytes sent by the client.
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			log.Printf("read from %s failed: %v", conn.RemoteAddr(), err)
			return
		}

		// send(): echo the received bytes back to the client.
		data := buf[:n]
		for len(data) > 0 {
			written, err := conn.Write(data)
			if err != nil {
				log.Printf("write to %s failed: %v", conn.RemoteAddr(), err)
				return
			}
			if written == 0 {
				log.Printf("write to %s failed: %v", conn.RemoteAddr(), io.ErrShortWrite)
				return
			}
			data = data[written:]
		}
	}
}
