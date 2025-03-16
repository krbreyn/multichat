package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

const (
	TCPPort = ":1337"
	WSPort  = ":8080"
)

type Client struct {
	Conn ChatConn
	ID   int
	L    *rate.Limiter
}

type ChatConn interface {
	watchConn(client Client, out chan<- Message)
	writeConn(msg string) error
	closeConn()
	getClientIP() string
}

type Message struct {
	Kind MsgKind
	Conn ChatConn // clients shouldnt be sending messages containing themselves, should fix
	Txt  string
}

type MsgKind int

const (
	NewClient MsgKind = iota + 1
	DeadClient
	NewMsg
	NewDirectClientMsg
	ServerShutdown
)

func msgHandler(in chan Message, up chan Message) {
	conns := make(map[ChatConn]struct{})

	sendMessages := func(up chan Message, msg string) {
		for conn := range conns {
			err := conn.writeConn(msg)
			if err != nil {
				up <- Message{Kind: DeadClient, Conn: conn, Txt: ""}
			}
		}
	}

	sendDirectMessage := func(up chan Message, target ChatConn, msg string) {
		for conn := range conns {
			if conn == target {
				err := conn.writeConn(msg)
				if err != nil {
					up <- Message{Kind: DeadClient, Conn: conn, Txt: ""}
				}
			}
		}
	}

handlerLoop:
	for {
		msg := <-in

		switch msg.Kind {

		case NewClient:
			conns[msg.Conn] = struct{}{}
			go sendMessages(up, msg.Txt)

		case DeadClient:
			msg.Conn.closeConn()
			delete(conns, msg.Conn)
			go sendMessages(up, msg.Txt)

		case NewMsg:
			go sendMessages(up, msg.Txt)

		case NewDirectClientMsg:
			go sendDirectMessage(up, msg.Conn, msg.Txt)

		case ServerShutdown:
			sendMessages(up, msg.Txt)
			for conn := range conns {
				conn.closeConn()
			}
			break handlerLoop
		}
	}
}

func server(msgs chan Message, logOut chan string) {
	clients := make(map[ChatConn]Client)
	var numClients int

	handlerChan := make(chan Message, 100)
	go msgHandler(handlerChan, msgs)

	logOut <- "started server\n"

serverLoop:
	for {
		msg := <-msgs
		switch msg.Kind {

		case NewClient:
			c := Client{
				Conn: msg.Conn,
				ID:   numClients,
				L:    rate.NewLimiter(rate.Every(500*time.Millisecond), 1),
			}
			clients[msg.Conn] = c
			numClients++
			go c.Conn.watchConn(c, msgs)

			msg.Txt = fmt.Sprintf("user %d has joined\n", clients[msg.Conn].ID)
			handlerChan <- msg

		case DeadClient:
			logOut <- fmt.Sprintf("disconnected %s\n", msg.Conn.getClientIP())
			delete(clients, msg.Conn)
			handlerChan <- msg

		case NewMsg:
			newTxt := fmt.Sprintf("user %d > %s", clients[msg.Conn].ID, msg.Txt)
			handlerChan <- Message{
				Kind: msg.Kind,
				Conn: msg.Conn,
				Txt:  newTxt,
			}
			logOut <- newTxt

		case NewDirectClientMsg:
			handlerChan <- msg

		case ServerShutdown:
			handlerChan <- msg
			break serverLoop
		}
	}
}

func main() {
	serverChan := make(chan Message, 100)
	logChan := make(chan string, 100)

	TCPListener, err := net.Listen("tcp", TCPPort)
	if err != nil {
		panic(err)
	}
	defer TCPListener.Close()

	go acceptTCPConns(TCPListener, serverChan, logChan)
	fmt.Printf("TCP running on %s\n", TCPPort)

	go acceptWSConns(WSPort, serverChan, logChan)
	fmt.Printf("WS running on %s\n", WSPort)

	go server(serverChan, logChan)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		serverChan <- Message{
			Kind: ServerShutdown,
			Conn: nil,
			Txt:  "The server is shutting down now!\n",
		}
		done <- true
	}()

	go func() {
		for msg := range logChan {
			fmt.Print(msg)
		}
	}()

	<-done
	time.Sleep(100 * time.Millisecond)
}
