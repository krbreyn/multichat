package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type TCPClient struct {
	Conn io.ReadWriteCloser
	IP   string
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

		if !client.L.Allow() {
			if time.Since(lastRateLimitMessage) > rateLimitPeriod {
				lastRateLimitMessage = time.Now()
				out <- Message{
					Kind: NewDirectClientMsg,
					Conn: client.Conn,
					Txt:  fmt.Sprintln("You are being rate limited!"),
				}
			}
			continue
		}

		out <- Message{
			Kind: NewMsg,
			Conn: client.Conn,
			Txt:  msg,
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
	return c.IP
}

func acceptTCPConns(listener net.Listener, serverChan chan<- Message, logOut chan<- string) {
	for {
		conn, err := listener.Accept()
		c_ip := conn.RemoteAddr().(*net.TCPAddr).IP.String()

		if err != nil {
			logOut <- fmt.Sprintf("%s failed to connect\n", c_ip)
		}

		tc := TCPClient{conn, c_ip}
		logOut <- fmt.Sprintf("accepted tcp conn from %s\n", c_ip)
		serverChan <- Message{
			Kind: NewClient,
			Conn: &tc,
			Txt:  "",
		}
	}
}
