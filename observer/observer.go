package observer

import (
	"encoding/json"
	"sync"

	"github.com/ccppluagopy/zed"
)

//OBServers ...
type OBServers struct {
	sync.Mutex
	name    string
	servers map[string]*ObserverServer //key:name
}

/*type EventMap struct {
	sync.RWMutex
	EventClient map[string]map[*zed.TcpClient]bool
}*/

//ObserverServer ...
type ObserverServer struct {
	OBDelaget
	Name     string
	Addr     string
	EventMap map[string]map[*zed.TcpClient]bool
}

var (
	observers = &OBServers{
		servers: make(map[string]*ObserverServer),
	}
)

//GetServer ...
func (observers *OBServers) GetServer(name string) *ObserverServer {
	observers.Lock()
	defer observers.Unlock()
	return observers.servers[name]
}

//AddServer ...
func (observers *OBServers) AddOBServer(name string, server *ObserverServer) {
	observers.Lock()
	defer observers.Unlock()
	if _, ok := observers.servers[name]; !ok {
		observers.servers[name] = server
	} else {
		zed.ZLog("OBServers AddServer Error: %s has been exist!", name)
	}

}

//DeleServer delete ObserverServer by name
func (observers *OBServers) DeleServer(name string) {
	observers.Lock()
	defer observers.Unlock()
	delete(observers.servers, name)
}

//-------------------------------------------------------------------------------ObserverServer

//NewObserverServer  creat a new ObserverServer
func NewOBServer(name string) *ObserverServer {
	if observer := observers.GetServer(name); observer == nil {
		tcpserver := zed.NewTcpServer(name)
		observer = &ObserverServer{
			Name:     name,
			EventMap: make(map[string]map[*zed.TcpClient]bool),
		}

		tcpserver.SetDelegate(observer)
		//observer.Server.SetMsgFilter(func(msg *zed.NetMsg) bool { return true })

		observers.AddOBServer(name, observer)
		return observer
	} else {
		zed.ZLog("NewObserverServer Error: %s has been exist.", name)
	}

	return nil
}

//handle heartbeat req
func (observer *ObserverServer) handleHeartBeat(client *zed.TcpClient) bool {
	//zed.ZLog("ObserverServer handleHeartBeatReq")
	client.SendMsgAsync(NewNetMsg(&OBMsg{
		OP: HEARTBEAT_RSP,
	}))
	return true
}

//handle regist req
func (observer *ObserverServer) handleRegist(event string, client *zed.TcpClient) bool {
	zed.ZLog("===== 000000  ObserverServer handleRegist %s ", event)

	if event == EventNull {
		client.SendMsgAsync(NewNetMsg(&OBMsg{
			OP:    REGIST_RSP,
			Event: ErrEventFlag,
			Data:  []byte(ErrRegistEventNull),
		}))

		//zed.ZLog("ObserverServer handleRegist 111")
		return true
	}

	events, ok := observer.EventMap[event]
	if !ok {
		events = make(map[*zed.TcpClient]bool)
		observer.EventMap[event] = events
	}

	events[client] = true

	client.SendMsgAsync(NewNetMsg(&OBMsg{
		OP: REGIST_RSP,
	}))
	client.AddCloseCB(event, func(c *zed.TcpClient) {
		delete(events, c)
	})

	//zed.ZLog("ObserverServer handleRegist 222")

	return true
}

//handle unregist req
func (observer *ObserverServer) handleUnregist(event string, client *zed.TcpClient) bool {
	zed.ZLog("===== 000000  ObserverServer handleUnregist  ", event)

	if event == EventNull {
		client.SendMsgAsync(NewNetMsg(&OBMsg{
			OP:    UNREGIST_RSP,
			Event: ErrEventFlag,
			Data:  []byte(ErrUnregistEventNull),
		}))

		//zed.ZLog("ObserverServer handleRegist 111")
		return true
	}

	events, ok := observer.EventMap[event]
	if !ok {
		client.SendMsgAsync(NewNetMsg(&OBMsg{
			OP:    UNREGIST_RSP,
			Event: ErrEventFlag,
			Data:  []byte(ErrUnegistNotRegisted),
		}))

		//zed.ZLog("ObserverServer handleRegist 222")
		return true
	}

	_, ok = events[client]
	if !ok {
		client.SendMsgAsync(NewNetMsg(&OBMsg{
			OP:    UNREGIST_RSP,
			Event: ErrEventFlag,
			Data:  []byte(ErrUnegistNotRegisted),
		}))

		//zed.ZLog("ObserverServer handleRegist 333")
		return true
	} else {
		client.SendMsgAsync(NewNetMsg(&OBMsg{
			OP: UNREGIST_RSP,
		}))
		client.RemoveCloseCB(event)
		//zed.ZLog("ObserverServer handleRegist 444")
	}

	return true
}

//handle publish req
func (observer *ObserverServer) handlePublish(event string, data []byte, client *zed.TcpClient) bool {
	zed.ZLog("==== 33333  ObserverServer handlePublish Event: %s, Data: %v", event, data)

	var (
		msg *zed.NetMsg = nil
	)

	client.SendMsgAsync(NewNetMsg(&OBMsg{
		OP: PUBLISH_RSP,
	}))

	clients, ok := observer.EventMap[event]
	if ok {
		msg = NewNetMsg(&OBMsg{
			OP:    PUBLISH_NOTIFY,
			Event: event,
			Data:  data,
		})
		for c, _ := range clients {
			c.SendMsgAsync(msg)
		}
		//zed.ZLog("ObserverServer handlePublish 222 Event: %s, Data: %v", event, data)
	}
	clients, ok = observer.EventMap[EventAll]
	if ok {
		if msg == nil {
			msg = NewNetMsg(&OBMsg{
				OP:    PUBLISH_NOTIFY,
				Event: event,
				Data:  data,
			})
		}
		for c, _ := range clients {
			c.SendMsgAsync(msg)
			//zed.Println("EventAll xxxx")
		}
		//zed.ZLog("ObserverServer handlePublish 333 Event: %s, Data: %v", event, data)
	}

	return true
}

//handle publish req
func (observer *ObserverServer) handlePublish2(event string, data []byte, client *zed.TcpClient) bool {
	zed.ZLog("==== 55555  ObserverServer handlePublish2 Event: %s, Data: %v", event, data)

	var (
		msg *zed.NetMsg = nil
	)

	client.SendMsgAsync(NewNetMsg(&OBMsg{
		OP: PUBLISH_RSP,
	}))

	clients, ok := observer.EventMap[event]
	if ok {
		msg = NewNetMsg(&OBMsg{
			OP:    PUBLISH_NOTIFY,
			Event: event,
			Data:  data,
		})
		for c, _ := range clients {
			c.SendMsgAsync(msg)
		}
	}
	clients, ok = observer.EventMap[EventAll]
	if ok {
		if msg == nil {
			msg = NewNetMsg(&OBMsg{
				OP:    PUBLISH_NOTIFY,
				Event: event,
				Data:  data,
			})
		}
		for c, _ := range clients {
			if c != client {
				c.SendMsgAsync(msg)
				zed.Println("EventAll xxxx")
			}
		}
		//zed.ZLog("ObserverServer handlePublish2 333 Event: %s, Data: %v", event, data)
	}

	return true
}

//HandleMsg ...
func (observer *ObserverServer) HandleMsg(msg *zed.NetMsg) {
	zed.ZLog("==== 66666  ObserverServer HandleMsg, Data: %s", string(msg.Data))
	observer.Lock()
	defer observer.Unlock()

	obmsg := OBMsg{}
	err := json.Unmarshal(msg.Data, &obmsg)
	if err != nil {
		obmsg.OP = OB_RSP_NONE
		obmsg.Event = ErrEventFlag
		obmsg.Data = []byte(ErrJsonUnmarshall)

		msg.Client.SendMsgAsync(NewNetMsg(&obmsg))
		return
	}

	switch obmsg.OP {
	case HEARTBEAT_REQ:
		observer.handleHeartBeat(msg.Client)
	case REGIST_REQ:
		observer.handleRegist(obmsg.Event, msg.Client)
		break
	case UNREGIST_REQ:
		observer.handleUnregist(obmsg.Event, msg.Client)
		break
	case PUBLISH_REQ:
		observer.handlePublish(obmsg.Event, obmsg.Data, msg.Client)
		break
	case PUBLISH2_REQ:
		observer.handlePublish2(obmsg.Event, obmsg.Data, msg.Client)
		break
	default:
		obmsg.OP = obmsg.OP
		obmsg.Event = ErrEventFlag
		obmsg.Data = []byte(ErrInvalidOP)
		msg.Client.SendMsgAsync(NewNetMsg(&obmsg))
		break
	}
}

func (observer *ObserverServer) Start(addr string) {
	//zed.NewCoroutine(func() {
	observer.Addr = addr
	observer.Server.Start(addr)
	//})
}

//delete obss's TcpServer
func (observer *ObserverServer) Stop() {
	observer.Lock()
	defer observer.Unlock()

	observers.DeleServer(observer.Name)
	observer.Server.Stop()
}
