package zed

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

type HandlerCB func(msg *NetMsg) bool

type msgtask struct {
	msgQ chan *NetMsg
}

func (task *msgtask) start4Sender() {
	var (
		msg      *NetMsg
		buf      []byte
		writeLen int
		err      error
	)

	for {
		for {
			msg = <-task.msgQ

			if msg == nil {
				return
			}

			if err = msg.Client.conn.SetWriteDeadline(time.Now().Add(WRITE_BLOCK_TIME)); err != nil {
				ZLog("Write Failed Cmd: %d, Len: %d, Buf: %s", msg.Cmd, msg.BufLen, string(msg.Buf))
				ZLog("Client(Id: %s, Addr: %s) SetWriteDeadline Error: %v!", msg.Client.Id, msg.Client.Addr, err)
				msg.Client.Stop()
			}

			buf = make([]byte, PACK_HEAD_LEN+len(msg.Buf))
			binary.LittleEndian.PutUint32(buf, uint32(len(msg.Buf)))
			binary.LittleEndian.PutUint32(buf[4:8], uint32(msg.Cmd))
			copy(buf[PACK_HEAD_LEN:], msg.Buf)

			writeLen, err = msg.Client.conn.Write(buf)
			ZLog("Write Success Cmd: %d, Len: %d, Buf: %s", msg.Cmd, msg.BufLen, string(msg.Buf))

			if err != nil || writeLen != len(buf) {
				msg.Client.Stop()
			}
		}

	}
}

func (task *msgtask) start4Handler(server *TcpServer) {
	var (
		msg *NetMsg
	)

	for {
		msg = <-task.msgQ

		if msg == nil {
			return
		}

		server.HandleMsg(msg)
	}
}

func (task *msgtask) stop() {
	if task.msgQ != nil {
		close(task.msgQ)
	}
}

type TcpServer struct {
	sync.RWMutex
	running    bool
	ClientNum  int
	listener   *net.TCPListener
	handlerMap map[CmdType]HandlerCB

	msgSendCorNum   int
	msgHandleCorNum int
	senders         []*msgtask
	handlers        []*msgtask

	clients     map[int]*TcpClient
	clientIdMap map[*TcpClient]ClientIDType
	idClientMap map[ClientIDType]*TcpClient
}

func (server *TcpServer) startSenders() *TcpServer {
	if server.msgSendCorNum != len(server.senders) {
		server.senders = make([]*msgtask, server.msgSendCorNum)
		for i := 0; i < server.msgSendCorNum; i++ {
			server.senders[i] = &msgtask{msgQ: make(chan *NetMsg, 5)}
			go server.senders[i].start4Sender()
			ZLog("TcpServer startSenders %d.", i)
		}
	}
	return server
}

func (server *TcpServer) stopSenders() *TcpServer {
	for i := 0; i < server.msgSendCorNum; i++ {
		server.senders[i].stop()
		ZLog("TcpServer stopSenders %d / %d.", i, server.msgSendCorNum)
	}

	return server
}

func (server *TcpServer) startHandlers() *TcpServer {
	if server.msgHandleCorNum != len(server.handlers) {
		server.handlers = make([]*msgtask, server.msgHandleCorNum)
		for i := 0; i < server.msgHandleCorNum; i++ {
			server.handlers[i] = &msgtask{msgQ: make(chan *NetMsg, 5)}
			go server.handlers[i].start4Handler(server)
			ZLog("TcpServer startHandlers %d.", i)
		}
	}
	return server
}

func (server *TcpServer) stopHandlers() *TcpServer {
	for i := 0; i < server.msgHandleCorNum; i++ {
		server.handlers[i].stop()
		ZLog("TcpServer stopHandlers %d / %d.", i, server.msgHandleCorNum)
	}

	return server
}

func (server *TcpServer) startListener(addr string) {
	defer Println("TcpServer Stopped.")
	var (
		tcpAddr *net.TCPAddr
		err     error
		client  *TcpClient
	)

	tcpAddr, err = net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		ZLog(fmt.Sprintf("ResolveTCPAddr error: %v\n", err) + GetStackInfo())
		//chStop <- "TcpServer Start Failed!"
		return
	}

	server.listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		ZLog(fmt.Sprintf("Listening error: %v\n", err) + GetStackInfo())
		//chStop <- "TcpServer Start Failed!"
		return
	}

	defer server.listener.Close()

	server.running = true

	ZLog(fmt.Sprintf("TcpServer Running on: %s", tcpAddr.String()))

	for {
		conn, err := server.listener.AcceptTCP()

		if !server.running {
			break
		}
		if err != nil {
			ZLog(fmt.Sprintf("Accept error: %v\n", err) + GetStackInfo())
		} else {
			client = newTcpClient(server, conn)
			if !client.start() {
				server.ClientNum = server.ClientNum + 1
				server.clients[client.Idx] = client
				client.AddCloseCB(0, func(client *TcpClient) {
					delete(server.clients, client.Idx)
				})
			}
		}
	}

}

func (server *TcpServer) Start(addr string) {
	if !server.running {
		server.startSenders()
		server.startHandlers()
		server.startListener(addr)
	}
}

func (server *TcpServer) Stop() {
	if server.running {
		defer PanicHandle(true, "TcpServer Stop().")

		for idx, client := range server.clients {
			client.Stop()
			delete(server.clients, idx)
		}
		server.stopHandlers()
		server.stopSenders()
		for k, _ := range server.handlerMap {
			delete(server.handlerMap, k)
		}
		for k, _ := range server.clientIdMap {
			delete(server.clientIdMap, k)
		}
		for k, _ := range server.idClientMap {
			delete(server.idClientMap, k)
		}

		server.listener.Close()
		server.running = false
	}
}

func (server *TcpServer) AddMsgHandler(cmd CmdType, cb HandlerCB) {
	ZLog("TcpServer AddMsgHandler", cmd, cb)

	server.handlerMap[cmd] = cb
}

func (server *TcpServer) RemoveMsgHandler(cmd CmdType, cb HandlerCB) {
	delete(server.handlerMap, cmd)
}

func (server *TcpServer) RelayMsg(msg *NetMsg) {
	if server.msgHandleCorNum == 0 {
		ZLog("TcpServer RelayMsg Error, msgHandleCorNum is 0.")
		return
	}
	server.handlers[msg.Client.Idx%server.msgHandleCorNum].msgQ <- msg
}

func (server *TcpServer) OnClientMsgError(msg *NetMsg) {
	msg.Client.SendMsg(msg)
}

func (server *TcpServer) HandleMsg(msg *NetMsg) {
	defer PanicHandle(true)

	cb, ok := server.handlerMap[msg.Cmd]
	if ok {
		if cb(msg) {
			return
		} else {
			ZLog(fmt.Sprintf("HandleMsg Error, Client(Id: %s, Addr: %s) Msg Cmd: %d, Buf: %v.", msg.Client.Id, msg.Client.Addr, msg.Cmd, msg.Buf))
		}
	} else {
		ZLog("No Handler For Cmd %d From Client(Id: %s, Addr: %s.", msg.Cmd, msg.Client.Id, msg.Client.Addr)
	}

	server.OnClientMsgError(msg)
}

func (server *TcpServer) SendMsg(msg *NetMsg) {
	if server.msgSendCorNum == 0 {
		ZLog("TcpServer SendMsg Error, msgSendCorNum is 0.")
		return
	}
	server.senders[msg.Client.Idx%server.msgSendCorNum].msgQ <- msg
}

func (server *TcpServer) GetClientById(id ClientIDType) *TcpClient {
	server.RLock()
	defer server.RUnlock()

	if c, ok := server.idClientMap[id]; ok {
		return c
	}

	return nil
}

func (server *TcpServer) AddClient(client *TcpClient) {
	if client.Id != NullId {
		server.Lock()
		defer server.Unlock()

		server.idClientMap[client.Id] = client
		server.clientIdMap[client] = client.Id
	}
}

func (server *TcpServer) RemoveClient(client *TcpClient) {
	if client.Id != NullId {
		server.Lock()
		defer server.Unlock()

		delete(server.idClientMap, client.Id)
		delete(server.clientIdMap, client)
	}
}

func (server *TcpServer) GetClientNum(client *TcpClient) (int, int) {
	return len(server.clientIdMap), server.ClientNum
}

func NewTcpServer(msgSendCorNum int, msgHandleCorNum int) *TcpServer {
	return &TcpServer{
		running:         false,
		ClientNum:       0,
		listener:        nil,
		handlerMap:      make(map[CmdType]HandlerCB),
		msgSendCorNum:   msgSendCorNum,
		msgHandleCorNum: msgHandleCorNum,
		clients:         make(map[int]*TcpClient),
		clientIdMap:     make(map[*TcpClient]ClientIDType),
		idClientMap:     make(map[ClientIDType]*TcpClient),
	}
}
