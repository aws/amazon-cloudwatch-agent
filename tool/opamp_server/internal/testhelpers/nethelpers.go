package testhelpers

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// GetAvailableLocalAddress finds an available local port and returns an endpoint
// describing it. The port is available for opening when this function returns
// provided that there is no race by some other code to grab the same port
// immediately.
func GetAvailableLocalAddress() string {
	ln, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		log.Fatalf("failed to get a free local port: %v", err)
	}
	// There is a possible race if something else takes this same port before
	// the test uses it, however, that is unlikely in practice.
	defer ln.Close()
	return ln.Addr().String()
}

func waitForPortToListen(port int) error {
	totalDuration := 5 * time.Second
	wait := 10 * time.Millisecond
	address := fmt.Sprintf("127.0.0.1:%d", port)

	ticker := time.NewTicker(wait)
	defer ticker.Stop()

	timeout := time.After(totalDuration)

	for {
		select {
		case <-ticker.C:
			conn, err := net.Dial("tcp", address)
			if err == nil && conn != nil {
				conn.Close()
				return nil
			}

		case <-timeout:
			return fmt.Errorf("failed to wait for port %d", port)
		}
	}
}

// HostPortFromAddr extracts host and port from a network address
func HostPortFromAddr(endpoint string) (host string, port int, err error) {
	sepIndex := strings.LastIndex(endpoint, ":")
	if sepIndex < 0 {
		return "", -1, errors.New("failed to parse host:port")
	}
	host, portStr := endpoint[:sepIndex], endpoint[sepIndex+1:]
	port, err = strconv.Atoi(portStr)
	return host, port, err
}

func WaitForEndpoint(endpoint string) {
	_, port, err := HostPortFromAddr(endpoint)
	if err != nil {
		log.Fatalln(err)
	}
	waitForPortToListen(port)
}
