package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

type TCPClient struct {
	Conn net.Conn
}

func (c *TCPClient) watchConn(client Client, out chan<- Message) {
	reader := bufio.NewReader(c.Conn)
	var lastRateLimitMessage time.Time
	rateLimitPeriod := 500 * time.Millisecond

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			out <- Message{
				Kind: DeadClient,
				Conn: client.Conn,
				Txt:  fmt.Sprintf("client %d has left\n", client.ID),
			}
			break
		}

		if strings.TrimSpace(msg) == "" {
			continue
		}

		if client.L.Allow() {
			out <- Message{
				Kind: NewMsg,
				Conn: client.Conn,
				Txt:  msg,
			}
		} else {
			if time.Since(lastRateLimitMessage) > rateLimitPeriod {
				lastRateLimitMessage = time.Now()
				out <- Message{
					Kind: NewDirectClientMsg,
					Conn: client.Conn,
					Txt:  fmt.Sprintln("You are being rate limited!"),
				}
			}
		}
	}
}

func (c *TCPClient) writeConn(msg string) error {
	_, err := c.Conn.Write([]byte(msg))
	return err
}

func (c *TCPClient) closeConn() {
	c.Conn.Close()
}

func (c *TCPClient) getClientIP() string {
	return c.Conn.RemoteAddr().(*net.TCPAddr).IP.String()
}

func acceptTCPConns(listener net.Listener, serverChan chan<- Message, logOut chan<- string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logOut <- fmt.Sprintf("%s failed to connect\n", conn.RemoteAddr().(*net.TCPAddr).IP.String())
		}
		tc := TCPClient{conn}
		logOut <- fmt.Sprintf("accepted tcp conn from %s\n", tc.getClientIP())
		serverChan <- Message{
			Kind: NewClient,
			Conn: &tc,
			Txt:  "",
		}
	}
}
