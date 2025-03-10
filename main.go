package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

// TODO - grab and print local ip on startup so you know what to connect to. also say 'localhost' for clarity
// TODO - server client

const (
	port      = ":8080"
	spamDelay = "1.0"
)

type Client struct {
	Conn    ChatConn
	ID      int
	LastMsg time.Time
}

type ChatConn interface {
	watchConn(client Client, out chan Message)
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
	ServerShutdown
)

type TCPClient struct {
	Conn net.Conn
}

func (t *TCPClient) watchConn(client Client, out chan Message) {
	reader := bufio.NewReader(t.Conn)
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
		out <- Message{
			Kind: NewMsg,
			Conn: client.Conn,
			Txt:  msg,
		}
	}
}

func (t *TCPClient) writeConn(msg string) error {
	_, err := t.Conn.Write([]byte(msg))
	return err
}

func (t *TCPClient) closeConn() {
	t.Conn.Close()
}

func (t *TCPClient) getClientIP() string {
	return t.Conn.RemoteAddr().(*net.TCPAddr).IP.String()
}

func acceptTCPConns(listener net.Listener, serverChan chan Message, logOut chan string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logOut <- fmt.Sprintf("%s failed to connect\n", conn.RemoteAddr().(*net.TCPAddr).IP.String())
		}
		serverChan <- Message{
			Kind: NewClient,
			Conn: &TCPClient{conn},
			Txt:  "",
		}
	}
}

// func watchWSConn() {

// }

// func acceptWSConns() {

// }

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

		}
	}

}

func server(msgs chan Message, logOut chan string) {
	clients := make(map[ChatConn]Client)
	var numClients int

	handlerChan := make(chan Message)
	go msgHandler(handlerChan, msgs)

	logOut <- "started server\n"

	for {
		msg := <-msgs
		switch msg.Kind {

		case NewClient:
			c := Client{Conn: msg.Conn, ID: numClients, LastMsg: time.Now()}
			clients[msg.Conn] = c
			numClients++
			go c.Conn.watchConn(c, msgs)

			logOut <- fmt.Sprintf("accepted %s\n", msg.Conn.getClientIP())
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

		case ServerShutdown:

		}
	}
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	fmt.Printf("running on %s\n", port)

	serverChan := make(chan Message)
	logChan := make(chan string)

	go acceptTCPConns(listener, serverChan, logChan)

	go server(serverChan, logChan)

	for msg := range logChan {
		fmt.Print(msg)
	}
}

// TODO commands typed into server console
// func handleServerCommand(cmd string) string {
// 	return cmd
// }

// TODO irc-like commands that begin with /
// func handleClientCommand(cmd string) string {
// 	return cmd
// }
