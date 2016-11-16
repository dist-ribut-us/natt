package main

import (
	"fmt"
	"github.com/dist-ribut-us/natt/igdp"
	"github.com/dist-ribut-us/rnet"
	"time"
)

// uses igdn to aquire the external IP and listens on port 1234 for 1 minute,
// but does not open the port. Useful to see how long the port will remain open

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
