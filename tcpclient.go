package zed

import (
	//"encoding/binary"
	"fmt"
	//"io"

	"net"
	//"sync"
	//"time"
)

type AsyncMsg struct {
	msg *NetMsg
	cb  func()
}

func (client *TcpClient) Info() string {
	return fmt.Sprintf("Client(ID: %v-Addr: %s)", client.ID, client.Addr)
}

func (client *TcpClient) AddCloseCB(key interface{}, cb ClientCloseCB) {
	client.Lock()
	defer client.Unlock()
	if client.running {
		client.closeCB[key] = cb
	}
}

func (client *TcpClient) GetConn() *net.TCPConn {
	return client.conn
}

func (client *TcpClient) RemoveCloseCB(key interface{}) {
	client.Lock()
	defer client.Unlock()
	if client.running {
		delete(client.closeCB, key)
	}
}

func (client *TcpClient) IsRunning() bool {
	return client.running
}

func (client *TcpClient) Stop() {
	if client.running {
		go func() {
			client.Lock()
			defer client.Unlock()

			if client.running {
				client.running = false

				if client.parent.showClientData {
					ZLog("[Stop_0] %s", client.Info())
				}
				LogStackInfo()

				client.conn.Close()
				//client.conn.SetLinger(0)

				if client.chSend != nil {
					close(client.chSend)
					client.chSend = nil
				}

				for _, cb := range client.closeCB {
					cb(client)
				}

				for key, _ := range client.closeCB {
					delete(client.closeCB, key)
				}

				if client.parent.showClientData {
					ZLog("[Stop_1] %s", client.Info())
				}

			}
		}()
	}
}

func (client *TcpClient) writer() {
	parent := client.parent
	if client.chSend != nil {
		for {
			if asyncMsg, ok := <-client.chSend; ok {
				if !func() bool {
					client.Lock()
					defer client.Unlock()
					defer func() {
						if err := PanicHandle(true); err != nil {
							client.Stop()
							return
						}
					}()
					parent.SendMsg(client, asyncMsg.msg)
					if asyncMsg.cb != nil {
						asyncMsg.cb()
					}
					return true
				}() {
					return
				}
			} else {
				//Println("========= writer stop()")
				break
			}
		}
	}
}

func (client *TcpClient) SendMsg(msg *NetMsg) {
	ZLog("[Send_1] %s Cmd: %d Len: %d", client.Info(), msg.Cmd, msg.Len)
	client.Lock()
	defer client.Unlock()

	client.parent.SendMsg(client, msg)

	//client.SendMsgAsync(msg)
	/*
			var (
				writeLen = 0
				buf      []byte
				err      error
			)

			if msg.Len > client.parent.maxPackLen {
				ZLog("SendMsg Err: Body Len(%d) > MAXPACK_LEN(%d)", msg.Len, client.parent.maxPackLen)
				goto Exit
			}

			if err := (*client.conn).SetWriteDeadline(time.Now().Add(client.parent.sendBlockTime)); err != nil {
				ZLog("%s SetWriteDeadline Err: %v.", client.Info(), err)
				goto Exit
			}

			msg.Client = client
			buf = make([]byte, PACK_HEAD_LEN+msg.Len)
			binary.LittleEndian.PutUint32(buf, uint32(msg.Len))
			binary.LittleEndian.PutUint32(buf[4:8], uint32(msg.Cmd))
			if msg.Len > 0 {
				copy(buf[PACK_HEAD_LEN:], msg.Data)
			}

			writeLen, err = client.conn.Write(buf)

			if dataOutSupervisor != nil {
				dataOutSupervisor(msg)
			} else if server.showClientData {
				ZLog("[Send] %s Cmd: %d, Len: %d, Data: %s", client.Info(), msg.Cmd, msg.Len, string(msg.Data))
			}

			if err == nil && writeLen == len(buf) {
				return
			}

		Exit:
			client.Stop()*/

}

func (client *TcpClient) SendMsgAsync(msg *NetMsg, argv ...interface{}) {
	ZLog("[Send_0] %s Cmd: %d Len: %d", client.Info(), msg.Cmd, msg.Len)

	client.Lock()
	defer client.Unlock()
	if client.running {

		asyncmsg := &AsyncMsg{
			msg: msg,
			cb:  nil,
		}

		if len(argv) > 0 {
			if cb, ok := (argv[0]).(func()); ok {
				asyncmsg.cb = cb
			}
		}
		if client.chSend != nil {
			Println("aaaaaaa", client.Info(), msg.Cmd, msg.Len, client.chSend)
			client.chSend <- asyncmsg
			Println("bbbbbbb", client.Info(), msg.Cmd, msg.Len, client.chSend)
		}
	}
	ZLog("[Send_00] %s Cmd: %d Len: %d", client.Info(), msg.Cmd, msg.Len)
}

/*
func (client *TcpClient) ReadMsg() *NetMsg {
	var (
		head    = make([]byte, PACK_HEAD_LEN)
		readLen = 0
		err     error
		msg     *NetMsg
	)

	if err = (*client.conn).SetReadDeadline(time.Now().Add(client.parent.recvBlockTime)); err != nil {
		if client.parent.showClientData {
			ZLog("%s SetReadDeadline Err: %v.", client.Info(), err)
		}
		goto Exit
	}

	readLen, err = io.ReadFull(client.conn, head)
	if err != nil || readLen < PACK_HEAD_LEN {
		if client.parent.showClientData {
			ZLog("%s Read Head Err: %v %d.", client.Info(), err, readLen)
		}
		goto Exit
	}

	if err = (*client.conn).SetReadDeadline(time.Now().Add(client.parent.recvBlockTime)); err != nil {
		if client.parent.showClientData {
			ZLog("%s SetReadDeadline Err: %v.", client.Info(), err)
		}
		goto Exit
	}

	msg = &NetMsg{
		Cmd:    CmdType(binary.LittleEndian.Uint32(head[4:8])),
		Len:    int(binary.LittleEndian.Uint32(head[0:4])),
		Client: client,
	}
	if msg.Len > client.parent.maxPackLen {
		ZLog("Read Body Err: Body Len(%d) > MAXPACK_LEN(%d)", msg.Len, client.parent.maxPackLen)
		goto Exit
	}
	if msg.Len > 0 {
		msg.Data = make([]byte, msg.Len)
		readLen, err := io.ReadFull(client.conn, msg.Data)
		if err != nil || readLen != int(msg.Len) {
			if client.parent.showClientData {
				ZLog("%s Read Body Err: %v.", client.Info(), err)
			}
			goto Exit
		}
	}

	return msg

Exit:
	return nil
}
*/
func (client *TcpClient) reader() {
	var (
		/*head    = make([]byte, PACK_HEAD_LEN)
		readLen = 0
		err     error*/
		msg    *NetMsg
		parent = client.parent
	)

	for {
		/*if err = (*client.conn).SetReadDeadline(time.Now().Add(client.parent.recvBlockTime)); err != nil {
			if client.parent.showClientData {
				ZLog("%s SetReadDeadline Err: %v.", client.Info(), err)
			}
			goto Exit
		}

		readLen, err = io.ReadFull(client.conn, head)
		if err != nil || readLen < PACK_HEAD_LEN {
			if client.parent.showClientData {
				ZLog("%s Read Head Err: %v %d.", client.Info(), err, readLen)
			}
			goto Exit
		}

		if err = (*client.conn).SetReadDeadline(time.Now().Add(client.parent.recvBlockTime)); err != nil {
			if client.parent.showClientData {
				ZLog("%s SetReadDeadline Err: %v.", client.Info(), err)
			}
			goto Exit
		}

		msg = &NetMsg{
			Cmd:    CmdType(binary.LittleEndian.Uint32(head[4:8])),
			Len:    int(binary.LittleEndian.Uint32(head[0:4])),
			Client: client,
		}

		if msg.Len > 0 {
			msg.Data = make([]byte, msg.Len)
			readLen, err := io.ReadFull(client.conn, msg.Data)
			if err != nil || readLen != int(msg.Len) {
				if client.parent.showClientData {
					ZLog("%s Read Body Err: %v.", client.Info(), err)
				}
				goto Exit
			}
		}*/
		msg = parent.RecvMsg(client)
		if msg == nil {
			goto Exit
		}

		//LogInfo(LOG_IDX, client.Idx, "Recv Msg %s Cmd: %d, Len: %d, Data: %s", client.Info(), msg.Cmd, msg.Len, string(msg.Data))

		parent.HandleMsg(msg)
	}

Exit:
	client.Stop()
	//LogInfo(LOG_IDX, client.Idx, "reader Exit %s", client.Info())
}

/*func (client *TcpClient) ConnectTo(addr string) bool {
	var err error
	client.conn, err = net.Dial("tcp", addr)
	if err != nil {
		ZLog("[ConnectTo] %s Error: %v", client.Info(), err)
		return false
	}

	return true
}*/

func (client *TcpClient) StartReader() {
	NewCoroutine(func() {
		client.reader()
	})
}

func (client *TcpClient) StartWriter() {
	NewCoroutine(func() {
		client.writer()
	})
}

func (client *TcpClient) start() bool {
	if err := client.conn.SetKeepAlive(true); err != nil {
		if client.parent.showClientData {
			ZLog("%s SetKeepAlive Err: %v.", client.Info())
		}
		return false
	}

	if err := client.conn.SetKeepAlivePeriod(client.parent.aliveTime); err != nil {
		if client.parent.showClientData {
			ZLog("%s SetKeepAlivePeriod Err: %v.", client.Info(), err)
		}
		return false
	}

	if err := (*client.conn).SetReadBuffer(client.parent.recvBufLen); err != nil {
		if client.parent.showClientData {
			ZLog("%s SetReadBuffer Err: %v.", client.Info(), err)
		}
		return false
	}
	if err := (*client.conn).SetWriteBuffer(client.parent.sendBufLen); err != nil {
		if client.parent.showClientData {
			ZLog("%s SetWriteBuffer Err: %v.", client.Info(), err)
		}
		return false
	}

	/*NewCoroutine(func() {
		client.writer()
	})*/
	client.StartWriter()
	client.StartReader()

	if client.parent.showClientData {
		ZLog("New Client Start %s", client.Info())
	}

	return true
}

func newTcpClient(parent *TcpServer, conn *net.TCPConn) *TcpClient {
	client := &TcpClient{
		conn:    conn,
		parent:  parent,
		ID:      NullID,
		Idx:     parent.ClientNum,
		Addr:    conn.RemoteAddr().String(),
		closeCB: make(map[interface{}]ClientCloseCB),
		chSend:  make(chan *AsyncMsg, 10),

		//Data:    nil,
		Valid:   false,
		running: true,
	}

	return client
}
