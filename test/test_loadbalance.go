package main

import (
	"fmt"
	lb "github.com/ccppluagopy/zed/loadbalance"
	"time"
)

func client(n int, stype string, stag string, addr string, addr2 string) {
	go func() {
		client := lb.NewLoadbalanceClient(addr)
		client.AddServer(stype, stag, addr2)
		for i := 0; i < n; i++ {
			client.Increament(stype, stag, 1)
		}
		time.Sleep(time.Second * 2)
		ret := client.GetServerAddr(stype)
		fmt.Println(stype, stag, addr, " -- over ---", ret.Addr, ret.Num)
	}()
}

func main() {
	addr := "127.0.0.1:8888"
	//server := lb.NewLoadbalanceServer(addr)
	lb.NewLoadbalanceServer(addr)
	client(1, "test server", "test server 1", addr, "0.0.0.0:1")
	client(2, "test server", "test server 2", addr, "0.0.0.0:2")
	client(3, "test server", "test server 3", addr, "0.0.0.0:3")

	time.Sleep(time.Hour)
}