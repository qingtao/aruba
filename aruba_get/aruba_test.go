package main

import (
	"testing"
)

var r3 = Rap3{
	Path:    "swarm.cgi",
	User:    "admin",
	Passwd:  `admin`,
	Cmd:     `%27show%20clients%20wired%27`,
	Timeout: 5,
	Filter:  true,
}

var ipTest = "101.0.133.1"

func TestArubaGetWired(t *testing.T) {
	r3.TrimMAC()
	cs, err := r3.GetClientsWired(ipTest)
	if err != nil {
		t.Fatalf("GetClientsWired: %#v\n", err)
		return
	}
	for i := 0; i < len(cs); i++ {
		t.Logf("show wired clients: \n%#v\n", cs[i])
	}
}
