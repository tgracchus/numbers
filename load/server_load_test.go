package load_test

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"testing"
)

func testServer(clientsNumber int, reqs int, address string) {
	var wg sync.WaitGroup
	wg.Add(clientsNumber)
	clients(&wg, clientsNumber, reqs, address)
	wg.Wait()
}

func TestServerTerminate(t *testing.T) {
	testServer(4, 400000, "localhost:4000")
}

func TestServerBaseline(t *testing.T) {
	testServer(5, 400000, "localhost:4000")
}

func TestServer_5connections_1clients_10000reqs(t *testing.T) {
	testServer(1, 1000, "localhost:4000")
}

func TestServer_5connections_5clients_1000reqs(t *testing.T) {
	testServer(5, 1000, "localhost:4000")
}

func TestServer_5connections_10clients_1000reqs(t *testing.T) {
	testServer(10, 1000, "localhost:4000")
}

func TestServer_5connections_5clients_10000reqs(t *testing.T) {
	testServer(5, 10000, "localhost:4000")
}

func TestServer_5connections_5clients_100000reqs(t *testing.T) {
	testServer(5, 100000, "localhost:4000")
}

func TestServer_5connections_5clients_1000000reqs(t *testing.T) {
	testServer(5, 1000000, "localhost:4000")
}

func TestServer_5connections_10clients_10000reqs(t *testing.T) {
	testServer(10, 10000, "localhost:4000")
}

func TestServer_5connections_10clients_100000reqs(t *testing.T) {
	testServer(10, 100000, "localhost:4000")
}

func TestServer_5connections_10clients_1000000reqs(t *testing.T) {
	testServer(10, 1000000, "localhost:4000")
}

func TestServer_5connections_50clients_10000reqs(t *testing.T) {
	testServer(50, 10000, "localhost:4000")
}

func TestServe_50connections_100clients_10000reqs(t *testing.T) {
	testServer(10, 10000, "localhost:4000")
}

func clients(wg *sync.WaitGroup, totalClients int, reqs int, address string) {
	var barrier sync.WaitGroup
	barrier.Add(1)
	for clientNumber := 0; clientNumber < totalClients; clientNumber++ {
		go client(wg, &barrier, clientNumber, reqs, address)
	}
	barrier.Done()
}

func sendTerminate(address string) {
	dialer := net.Dialer{KeepAlive: 15}
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		log.Printf("Client connection error: %s", err)
	}
	defer conn.Close()
	send(conn, "terminate\n")
}

func client(wg *sync.WaitGroup, barrier *sync.WaitGroup, clientNumber int, reqs int, address string) {
	barrier.Wait()
	dialer := net.Dialer{KeepAlive: 15}
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		log.Printf("Client %d connection error: %s", clientNumber, err)
		return
	}
	defer conn.Close()
	for i := 0; i < reqs; i++ {
		// send to socket
		number := fmt.Sprintf("%09d\n", rand.Intn(1000000000))
		err := send(conn, number)
		if err != nil {
			log.Printf("Client %d with error: %s", clientNumber, err)
			return
		}
	}
	wg.Done()
}

func send(conn net.Conn, msg string) error {
	_, err := fmt.Fprintf(conn, msg)
	if err != nil {
		return err
	}
	return nil
}
