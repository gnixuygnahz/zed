package main

import (
	"fmt"
	"github.com/ccppluagopy/zed"
	"github.com/ccppluagopy/zed/observer"
	"time"
)

func xxx(addr string, data string, n int) {
	for i := 0; i < 2; i++ {
		selfkey := fmt.Sprintf("%s-%d", addr, i)
		go func() {
			obsc := observer.NewOBClient(addr, selfkey, time.Second*20)
			if obsc == nil {
				fmt.Println("obsc is nil.....")
				return
			}

			event := "chat"
			obsc.Regist("xx", event, func(e interface{}, args []interface{}) {
				msg, ok := args[0].([]byte)
				if ok {
					fmt.Printf("--- %s recv chatMsg: %s %s\n", selfkey, e, string(msg))
				} else {
					fmt.Printf("xxx %s recv chatMsg: %s %s\n", selfkey, e, string(msg))
				}
			})

			time.Sleep(time.Second * 1)
			obsc.PublishAll(event, []byte(selfkey))
			time.Sleep(time.Second * 1)
			obsc.Stop()
		}()
	}
}

func main() {
	zed.SetMutexDebug(true)
	mgrAddr := "127.0.0.1:6666"
	nodeAddr1 := "127.0.0.1:7777"
	nodeAddr2 := "127.0.0.1:8888"
	nodeAddr3 := "127.0.0.1:9999"
	go observer.NewOBClusterMgr(mgrAddr)
	go observer.NewOBClusterNode(mgrAddr, nodeAddr1, time.Second*25).Start()
	go observer.NewOBClusterNode(mgrAddr, nodeAddr2, time.Second*25).Start()
	go observer.NewOBClusterNode(mgrAddr, nodeAddr3, time.Second*25).Start()

	time.Sleep(time.Second * 3)
	go xxx(nodeAddr1, "node 111", 1)
	go xxx(nodeAddr2, "node 222", 2)
	go xxx(nodeAddr3, "node 333", 3)

	time.Sleep(time.Hour)
}
