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
				LogInfo(LOG_IDX, msg.Client.Idx, "Write Failed Cmd: %d, Len: %d, Buf: %s", msg.Cmd, msg.BufLen, string(msg.Buf))
				LogError(LOG_IDX, msg.Client.Idx, "Client(Id: %s, Addr: %s) SetWriteDeadline Error: %v!", msg.Client.Id, msg.Client.Addr, err)
				msg.Client.Stop()
			}

			buf = make([]byte, PACK_HEAD_LEN+len(msg.Buf))
			binary.LittleEndian.PutUint32(buf, uint32(len(msg.Buf)))
			binary.LittleEndian.PutUint32(buf[4:8], uint32(msg.Cmd))
			copy(buf[PACK_HEAD_LEN:], msg.Buf)

			writeLen, err = msg.Client.conn.Write(buf)
			LogInfo(LOG_IDX, msg.Client.Idx, "Write Success Cmd: %d, Len: %d, Buf: %s", msg.Cmd, msg.BufLen, string(msg.Buf))

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

type TcpServer struct {
	sync.RWMutex
	running         bool
	ClientNum       int
	listener        *net.TCPListener
	handlerMap      map[CmdType]HandlerCB
	msgSendCorNum   int
	msgHandleCorNum int

	clientIdMap map[*TcpClient]ClientIDType
	idClientMap map[ClientIDType]*TcpClient

	senders  []*msgtask
	handlers []*msgtask
}

func (server *TcpServer) startSenders() *TcpServer {
	if server.msgSendCorNum != len(server.senders) {
		server.senders = make([]*msgtask, server.msgSendCorNum)
		for i := 0; i < server.msgSendCorNum; i++ {
			server.senders[i] = &msgtask{msgQ: make(chan *NetMsg, 5)}
			go server.senders[i].start4Sender()
		}
	}
	return server
}

func (server *TcpServer) startHandlers() *TcpServer {
	if server.msgHandleCorNum != len(server.senders) {
		server.handlers = make([]*msgtask, server.msgHandleCorNum)
		for i := 0; i < server.msgHandleCorNum; i++ {
			server.handlers[i] = &msgtask{msgQ: make(chan *NetMsg, 5)}
			go server.handlers[i].start4Handler(server)
		}
	}
	return server
}

func (server *TcpServer) Start(addr string) *TcpServer {
	if server.running {
		return server
	}

	//go
	func() {
		var (
			tcpAddr *net.TCPAddr
			err     error
		)

		tcpAddr, err = net.ResolveTCPAddr("tcp4", addr)
		if err != nil {
			LogError(LOG_IDX, LOG_IDX, fmt.Sprintf("ResolveTCPAddr error: %v\n", err)+GetStackInfo())
			//chStop <- "TcpServer Start Failed!"
			return
		}

		server.listener, err = net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			LogError(LOG_IDX, LOG_IDX, fmt.Sprintf("Listening error: %v\n", err)+GetStackInfo())
			//chStop <- "TcpServer Start Failed!"
			return
		}

		defer server.listener.Close()

		server.running = true

		LogInfo(LOG_IDX, LOG_IDX, fmt.Sprintf("TcpServer Running on: %s", tcpAddr.String()))

		for {
			conn, err := server.listener.AcceptTCP()

			if !server.running {
				break
			}
			if err != nil {
				LogInfo(LOG_IDX, LOG_IDX, fmt.Sprintf("Accept error: %v\n", err)+GetStackInfo())
			} else {
				if !newTcpClient(server, conn).start() {
					server.ClientNum = server.ClientNum - 1
				}
			}
		}
	}()

	return server
}

func (server *TcpServer) Stop() {
	server.running = false
	server.listener.Close()

	LogInfo(LOG_IDX, LOG_IDX, "[ShutDown] TcpServer Stop!")
}

func (server *TcpServer) AddMsgHandler(cmd CmdType, cb HandlerCB) {
	LogInfo(LOG_IDX, LOG_IDX, "TcpServer AddMsgHandler", cmd, cb)

	server.handlerMap[cmd] = cb
}

func (server *TcpServer) RemoveMsgHandler(cmd CmdType, cb HandlerCB) {
	delete(server.handlerMap, cmd)
}

func (server *TcpServer) RelayMsg(msg *NetMsg) {
	if server.msgHandleCorNum == 0 {
		LogError(LOG_IDX, msg.Client.Idx, "TcpServer RelayMsg Error, msgHandleCorNum is 0.")
		return
	}
	server.handlers[msg.Client.Idx%server.msgHandleCorNum].msgQ <- msg
}

func (server *TcpServer) HandleMsg(msg *NetMsg) {
	cb, ok := server.handlerMap[msg.Cmd]
	if ok {
		if cb(msg) {
			return
		}
	} else {
		LogInfo(LOG_IDX, msg.Client.Idx, "No Handler For Cmd %d From Client(Id: %s, Addr: %s.", msg.Cmd, msg.Client.Id, msg.Client.Addr)
	}

Err:
	msg.Client.SendMsg(msg)
}

func (server *TcpServer) SendMsg(msg *NetMsg) {
	if server.msgSendCorNum == 0 {
		LogError(LOG_IDX, msg.Client.Idx, "TcpServer SendMsg Error, msgSendCorNum is 0.")
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
		clientIdMap:     make(map[*TcpClient]ClientIDType),
		idClientMap:     make(map[ClientIDType]*TcpClient),
	}
}
