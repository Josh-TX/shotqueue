package main

import (
	"flag"
	"fmt"
	"net"
	"os"
)

func main() {
	ip := flag.String("ip", "", "IP address of the ATEM (or mock-atem) switcher")
	flag.Parse()

	if *ip == "" || net.ParseIP(*ip) == nil {
		fmt.Fprintln(os.Stderr, "usage: go-atem-listener --ip=<switcher ip>")
		os.Exit(1)
	}

	runForever(*ip)
}
