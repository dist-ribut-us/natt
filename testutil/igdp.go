package main

import (
	"fmt"
	"github.com/dist-ribut-us/crypto"
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

	p := crypto.RandUint16()
	for ; p < 1000; p = crypto.RandUint16() {
	}
	port := fmt.Sprintf(":%d", p)

	srv, err := rnet.RunNew(port, &packeter{})
	if err != nil {
		fmt.Println(err)
		return
	}

	externalIp, err := igdp.GetExternalIP()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = igdp.AddPortMapping(srv.Port(), srv.Port())
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Listening on", externalIp, ":", srv.Port())

	time.Sleep(time.Minute)
}
