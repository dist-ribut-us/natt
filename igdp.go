package main

// opens port maps external port 1234 to local port 1234 and listens on that
// port for 1 minute.

import (
	"fmt"
	"github.com/dist-ribut-us/natt/igdp"
	"github.com/dist-ribut-us/rnet"
	"time"
)

type packeter struct{}

func (p *packeter) Receive(b []byte, a *rnet.Addr) {
	fmt.Println("From:", a.String())
	fmt.Println(string(b))
}

func main() {
	err := igdp.Setup()
	if err != nil {
		fmt.Println(err)
		return
	}
	externalIp, err := igdp.GetExternalIP()
	igdp.AddPortMapping(1234, 1234)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Listening on", externalIp, ":1234")
	_, err = rnet.RunNew(":1234", &packeter{})
	if err != nil {
		fmt.Println(err)
		return
	}
	time.Sleep(time.Minute)
}
