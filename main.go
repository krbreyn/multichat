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
	Conn    net.Conn
	Id      int
	LastMsg time.Time
}

type Message struct {
	Kind MsgKind
	Conn net.Conn
	Txt  string
}

type MsgKind int

const (
	NewClient MsgKind = iota + 1
	DeadClient
	NewMsg
	ServerShutdown
)

func watchConn(client Client, out chan Message) {
	reader := bufio.NewReader(client.Conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			out <- Message{
				Kind: DeadClient,
				Conn: client.Conn,
				Txt:  fmt.Sprintf("client %d has left\n", client.Id),
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

func acceptConnections(listener net.Listener, serverChan chan Message, logOut chan string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logOut <- fmt.Sprintf("%s failed to connect\n", conn.RemoteAddr().(*net.TCPAddr).IP.String())
		}
		serverChan <- Message{
			Kind: NewClient,
			Conn: conn,
			Txt:  "",
		}
	}
}

func msgHandler(in chan Message, up chan Message) {
	conns := make(map[net.Conn]struct{})

	sendMessages := func(up chan Message, msg string) {
		for conn := range conns {
			_, err := conn.Write([]byte(msg))
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
			msg.Conn.Close()
			delete(conns, msg.Conn)
			go sendMessages(up, msg.Txt)

		case NewMsg:
			go sendMessages(up, msg.Txt)

		}
	}

}

func server(msgs chan Message, logOut chan string) {
	clients := make(map[net.Conn]Client)
	var numClients int

	handlerChan := make(chan Message)
	go msgHandler(handlerChan, msgs)

	logOut <- "started server\n"

	for {
		msg := <-msgs
		switch msg.Kind {

		case NewClient:
			c := Client{Conn: msg.Conn, Id: numClients, LastMsg: time.Now()}
			clients[msg.Conn] = c
			numClients++
			go watchConn(c, msgs)

			logOut <- fmt.Sprintf("accepted %s\n", msg.Conn.RemoteAddr().(*net.TCPAddr).IP.String())
			msg.Txt = fmt.Sprintf("user %d has joined\n", clients[msg.Conn].Id)
			handlerChan <- msg

		case DeadClient:
			logOut <- fmt.Sprintf("disconnected %s\n", msg.Conn.RemoteAddr().(*net.TCPAddr).IP.String())
			delete(clients, msg.Conn)
			handlerChan <- msg

		case NewMsg:
			newTxt := fmt.Sprintf("user %d > %s", clients[msg.Conn].Id, msg.Txt)
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

	go acceptConnections(listener, serverChan, logChan)

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
