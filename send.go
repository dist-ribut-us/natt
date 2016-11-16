package main

import (
	"fmt"
	"github.com/dist-ribut-us/rnet"
	"os"
	"strings"
)

// command line to send a simple message over udp to an IP and port

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Specify address and port to send to")
		return
	}

	msg := "Hello"
	if len(os.Args) > 2 {
		msg = strings.Join(os.Args[2:], " ")
	}

	addr, err := rnet.ResolveAddr(os.Args[1])
	if err != nil {
		panic(err)
	}

	srv, err := rnet.New(":0", nil)
	if err != nil {
		panic(err)
	}

	srv.Send([]byte(msg), addr)
	fmt.Println("Sent \""+msg+"\" to", addr)
}
