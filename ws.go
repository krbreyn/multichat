package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type WSClient struct {
	Conn       *websocket.Conn
	RemoteAddr string
}

func (c *WSClient) watchConn(client Client, out chan<- Message) {
	var lastRateLimitMessage time.Time
	rateLimitPeriod := 500 * time.Millisecond

	for {
		_, bmsg, err := c.Conn.Read(context.Background())
		if err != nil {
			out <- Message{
				Kind: DeadClient,
				Conn: client.Conn,
				Txt:  fmt.Sprintf("client %d has left\n", client.ID),
			}
			break
		}

		msg := string(bmsg)
		if strings.TrimSpace(msg) == "" {
			continue
		}

		if client.L.Allow() {
			out <- Message{
				Kind: NewMsg,
				Conn: client.Conn,
				Txt:  msg + "\n",
			}
		} else {
			if time.Since(lastRateLimitMessage) > rateLimitPeriod {
				lastRateLimitMessage = time.Now()
				out <- Message{
					Kind: NewDirectClientMsg,
					Conn: client.Conn,
					Txt:  "You are being rate limited!",
				}
			}
		}
	}

}

func (c *WSClient) writeConn(msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return c.Conn.Write(ctx, websocket.MessageText, []byte(msg))
}

func (c *WSClient) closeConn() {
	_ = c.Conn.Close(websocket.StatusNormalClosure, "connection closed")
}

func (c *WSClient) getClientIP() string {
	return c.RemoteAddr
}

func acceptWSConns(addr string, serverChan chan<- Message, logOut chan<- string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			logOut <- fmt.Sprintf("failed to upgrade to websocket: %v\n", err)
			return
		}

		clientIP := r.RemoteAddr

		wsClient := &WSClient{
			Conn:       conn,
			RemoteAddr: clientIP,
		}

		serverChan <- Message{
			Kind: NewClient,
			Conn: wsClient,
			Txt:  "",
		}

		logOut <- fmt.Sprintf("accepted ws conn from %s\n", clientIP)
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		logOut <- fmt.Sprintf("ws server failed: %v\n", err)
	}
}
