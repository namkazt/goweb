package gocore

import (
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/labstack/echo/v4"
	"github.com/mailru/easygo/netpoll"
	"io"
	"io/ioutil"
	"net"
	"sort"
	"sync"
	"runtime"
	"time"
	"bytes"
	"net/http"
)

const (
	WS_DEADLINE_DURATION_READ = 100 * time.Millisecond
	WS_DEADLINE_DURATION_WRITE = 100 * time.Millisecond
)

type AppWebSocket struct {
	app 						*App
	pool 						*Pool
	singleThreadProcess 		bool

	// callback
	OnOpen 						func(client *WSClient)
	OnClose 					func(uuid string)
	OnJoinChannel 				func(client *WSClient, channel *WSChannel)
	OnLeaveChannel 				func(uuid string, channel *WSChannel)
	OnMessage 					func(client *WSClient, data []byte)

	// users list ( combile with all channel )
	userLock 					sync.RWMutex
	users 						map[string]*WSClient
	globaOut 					chan []byte

	// channel list
	channelLock					sync.RWMutex
	channels 					[]*WSChannel
	nameChannels				map[string]*WSChannel
}

func NewAppWebSocket(app *App, wsRoute string, poolSize int, singleThreadProcess bool) *AppWebSocket{
	instance := &AppWebSocket{}
	instance.app = app
	instance.pool = NewPool(poolSize, 1, 1)
	instance.singleThreadProcess = singleThreadProcess
	instance.nameChannels = make(map[string]*WSChannel)
	instance.users = make(map[string]*WSClient)
	instance.globaOut = make(chan []byte, 1)

	instance.OnOpen = func(client *WSClient){}
	instance.OnClose = func(uuid string){}
	instance.OnJoinChannel = func(client *WSClient, channel *WSChannel){}
	instance.OnLeaveChannel = func(uuid string, channel *WSChannel){}
	instance.OnMessage = func(client *WSClient, data []byte){}

	instance.Init(wsRoute)

	go instance.globalBroadcaster()

	return instance
}

func (this*AppWebSocket) Init(wsRoute string) {
	if runtime.GOOS == "windows" {
		this.initWindows(wsRoute)
	} else {
		this.initLinux(wsRoute)
	}
}


func (this*AppWebSocket) globalBroadcaster() {
	for bts := range this.globaOut {
		this.userLock.RLock()
		for _, u := range this.users {
			u := u // For closure.
			this.pool.Schedule(func() {
				u.internalWrite(bts)
			})
		}
		this.userLock.RUnlock()
	}
}

func (this*AppWebSocket) initWindows(wsRoute string) {
	this.app.Echo().GET(wsRoute, func(c echo.Context) error{
		conn, _, _, err := ws.UpgradeHTTP(c.Request(), c.Response().Writer)
		if err != nil {
			Log().Error().Err(err).Msg("Upgrade WS Error")
			return echo.NewHTTPError(http.StatusInternalServerError, "Upgrade WS Error")
		}
		// register new client connected to this websocket
		this.registerClient(conn)
		return nil
	})

	go func() {
		for {
			this.userLock.Lock()
			for _, client := range this.users {
				if !client.IsReading() {
					safeConn := deadliner{client.conn.(net.Conn), time.Nanosecond}
					buf := make([]byte, 0)
					_, err := safeConn.Read(buf)
					if err == io.EOF || err == nil {
						// submit task for reading
						c := client
						c.SetReading()
						if this.singleThreadProcess {
							data, err := c.Read()
							if err != nil {
								removed := this.internalRemove(c.uuid)
								if removed {
									this.OnClose(c.uuid)
									Log().Info().Str("uuid", c.uuid ).Msg("Client disconnected")
								}
							}else if data != nil {
								this.OnMessage(c, data)
							}
						}else{
							this.pool.Schedule(func() {
								data, err := c.Read()
								if err != nil {
									this.Remove(c.uuid)
								}else if data != nil {
									this.OnMessage(c, data)
								}
							})
						}
					}
				}
			}
			this.userLock.Unlock()
		}
	}()
}

func (this*AppWebSocket) initLinux(wsRoute string) {
	// net poller
	poller, err := netpoll.New(nil)
	if err != nil {
		Log().Error().Err(err).Msg("Can't create new netpoll")
		return
	}

	// init route
	this.app.Echo().GET(wsRoute, func(c echo.Context) error {
		conn, _, _, err := ws.UpgradeHTTP(c.Request(), c.Response().Writer)
		if err != nil {
			Log().Error().Err(err).Msg("Upgrade WS Error")
			return echo.NewHTTPError(http.StatusInternalServerError, "Upgrade WS Error")
		}
		// register new client connected to this websocket
		client := this.registerClient(conn)
		// create netpoll event descriptor for conn
		readDesc := netpoll.Must(netpoll.HandleRead(conn))
		_ = poller.Start(readDesc, func(ev netpoll.Event) {
			if ev & (netpoll.EventReadHup | netpoll.EventHup) != 0 {
				// When ReadHup or Hup received, this mean that client has
				// closed at least write end of the connection or connections
				// itself. So we want to stop receive events about such conn
				// and remove it from the chat registry.
				_ = poller.Stop(readDesc)
				this.Remove(client.uuid)
				return
			}
			// Here we can read some new message from connection.
			// We can not read it right here in callback, because then we will
			// block the poller's inner loop.
			// We do not want to spawn a new goroutine to read single message.
			// But we want to reuse previously spawned goroutine.
			if this.singleThreadProcess {
				data, err := client.Read()
				if err != nil {
					_ = poller.Stop(readDesc)
					removed := this.internalRemove(client.uuid)
					if removed {
						this.OnClose(client.uuid)
						Log().Info().Str("uuid", client.uuid ).Msg("Client disconnected")
					}
				}else if data != nil {
					this.OnMessage(client, data)
				}
			}else {
				this.pool.Schedule(func() {
					data, err := client.Read()
					if err != nil {
						_ = poller.Stop(readDesc)
						this.Remove(client.uuid)
					}else if data != nil {
						this.OnMessage(client, data)
					}
				})
			}

		})

		return nil
	})
}

func (this*AppWebSocket) RangeChannel(callback func(channel *WSChannel)) {
	this.channelLock.RLock()
	for _, c := range this.channels {
		callback(c)
	}
	this.channelLock.RUnlock()
}

func (this*AppWebSocket) HasChannel(name string) bool {
	this.channelLock.RLock()
	_, has := this.nameChannels[name]
	this.channelLock.RUnlock()
	return has
}


func (this*AppWebSocket) AddChannel(channel *WSChannel) {
	this.channelLock.Lock()
	{
		this.channels = append(this.channels, channel)
		this.nameChannels[channel.name] = channel
	}
	this.channelLock.Unlock()

	Log().Info().Str("Name", channel.name ).Msg("Added new channel")
}

func (this*AppWebSocket) RemoveAllChannel() {
	this.channelLock.Lock()
	defer this.channelLock.Unlock()
	for _, c := range this.channels {
		c.Close()
	}
	this.channels = nil
	this.nameChannels = make(map[string]*WSChannel)
}

func (this*AppWebSocket) RemoveChannel(name string) bool{
	this.channelLock.RLock()
	channel, found := this.nameChannels[name]
	this.channelLock.RUnlock()
	if !found {
		return false
	}

	this.channelLock.Lock()
	delete(this.nameChannels, name)

	i := sort.Search(len(this.channels) - 1, func(i int) bool {
		return this.channels[i].name == name
	})
	this.channels[i] = this.channels[len(this.channels)-1]
	this.channels[len(this.channels)-1] = nil
	this.channels = this.channels[:len(this.channels)-1]

	this.channelLock.Unlock()

	channel.Close()
	Log().Info().Str("Name", channel.name ).Msg("Removed channel")
	return true
}

func (this*AppWebSocket) BroadcastInChannel(data []byte, channelName string) {
	this.channelLock.RLock()
	channel, has := this.nameChannels[channelName]
	this.channelLock.RUnlock()
	if has {
		channel.Broadcast(data)
	}
}


func (this*AppWebSocket) BroadcastInChannelH(data echo.Map, channelName string) {
	this.channelLock.RLock()
	channel, has := this.nameChannels[channelName]
	this.channelLock.RUnlock()
	if has {
		channel.BroadcastH(data)
	}
}

func (this*AppWebSocket) BroadcastAllChannel(data []byte) {
	this.channelLock.RLock()
	channels := this.channels
	this.channelLock.RUnlock()
	for _, channel := range channels {
		channel.Broadcast(data)
	}
}

func (this*AppWebSocket) BroadcastAllChannelH(data echo.Map) {
	this.channelLock.RLock()
	channels := this.channels
	this.channelLock.RUnlock()
	for _, channel := range channels {
		channel.BroadcastH(data)
	}
}

func (this*AppWebSocket) BroadcastGlobal(data []byte) {
	var buf bytes.Buffer
	w := wsutil.NewWriter(&buf, ws.StateServerSide, ws.OpText)
	w.Write(data)
	w.Flush()
	this.globaOut <- buf.Bytes()
}

func (this*AppWebSocket) BroadcastGlobalH(data echo.Map) {
	var buf bytes.Buffer
	w := wsutil.NewWriter(&buf, ws.StateServerSide, ws.OpText)
	b, err := json.Marshal(data)
	if err != nil {
		Log().Error().Err(err).Msg("Encode error")
		return
	}
	w.Write(b)
	w.Flush()
	this.globaOut <- buf.Bytes()
}

func (this*AppWebSocket) ClientJoinChannel(client *WSClient, channelName string) bool{
	client.io.RLock()
	if client.channel != nil {
		// remove client from old channel before add to new one
		client.channel.Remove(client.id)
	}
	client.io.RUnlock()

	// check if channel name is available
	this.channelLock.RLock()
	if _, has := this.nameChannels[channelName]; !has {
		this.channelLock.RUnlock()
		return false
	}
	this.channelLock.RUnlock()

	// add to new channel
	this.channelLock.Lock()
	this.nameChannels[channelName].AddClient(client)
	this.channelLock.Unlock()

	this.OnJoinChannel(client, this.nameChannels[channelName])

	return true
}

func (this*AppWebSocket) registerClient(conn net.Conn) *WSClient {
	client := &WSClient{
		server: this,
		conn: conn,
		RemoteAddress: conn.RemoteAddr().String(),
	}
	// save client to map
	this.userLock.Lock()
	{
		client.uuid = GetUniqueCode(16)
		this.users[client.uuid] = client
	}
	this.userLock.Unlock()

	this.OnOpen(client)
	Log().Info().Int("ID", client.id ).Msg("Client connected")

	return client
}

// remove user from global list
// it already take care if user in a channel then channel will remove user too
func (this*AppWebSocket) Remove(uuid string) bool{
	this.userLock.Lock()
	removed := this.internalRemove(uuid)
	this.userLock.Unlock()
	if removed {
		this.OnClose(uuid)
		Log().Info().Str("uuid", uuid ).Msg("Client disconnected")
	}
	return removed
}

func (this*AppWebSocket) internalRemove(uuid string) bool{
	// remove in global list
	if _, has := this.users[uuid]; !has {
		return false
	}
	user := this.users[uuid]
	defer func() { user = nil }()
	delete(this.users, uuid)
	// remove in channel
	channel := user.channel
	if channel != nil {
		idInChannel := user.id
		if _, has := this.nameChannels[channel.name]; !has {
			return false
		}
		return channel.Remove(idInChannel)
	}
	return true
}


// deadliner is a wrapper around net.Conn that sets read/write deadlines before
// every Read() or Write() call.
type deadliner struct {
	net.Conn
	t time.Duration
}

func (d deadliner) Write(p []byte) (int, error) {
	if err := d.Conn.SetWriteDeadline(time.Now().Add(d.t)); err != nil {
		return 0, err
	}
	return d.Conn.Write(p)
}

func (d deadliner) Read(p []byte) (int, error) {
	if err := d.Conn.SetReadDeadline(time.Now().Add(d.t)); err != nil {
		return 0, err
	}
	return d.Conn.Read(p)
}

//-------------------------------------------------------------
// CLIENT
//-------------------------------------------------------------
type WSClient struct {

	RemoteAddress 				string

	id 							int
	uuid 						string
	server 						*AppWebSocket

	channel 					*WSChannel

	io   						sync.RWMutex
	conn 						io.ReadWriteCloser
	reading 					bool

	context 					interface{}
}

func (c*WSClient) GetUUID() string{
	return c.uuid
}

func (c*WSClient) GetContext() interface{}{
	return c.context
}

func (c*WSClient) SetContext(context interface{}) {
	c.context = context
}

func (c*WSClient) ClearContext() {
	c.context = nil
}



func (c*WSClient) IsReading() bool{
	c.io.RLock()
	defer c.io.RUnlock()
	return c.reading
}


func (c*WSClient) SetReading() {
	c.io.Lock()
	defer c.io.Unlock()
	c.reading = true
}

func (c*WSClient) ClearReading() {
	c.io.Lock()
	defer c.io.Unlock()
	c.reading = false
}


func (c*WSClient) Read() ([]byte, error){
	data, err := c.internalRead()
	if err != nil {
		c.ClearReading()
		//Log().Error().Err(err).Msg("Error when reading from client.")
		_ = c.conn.Close()
		return nil, err
	}
	c.ClearReading()
	return data, nil
}

func (c*WSClient) internalRead() ([]byte, error){
	c.io.Lock()
	defer c.io.Unlock()

	c.conn.(net.Conn).SetDeadline(time.Now().Add(WS_DEADLINE_DURATION_READ))

	h, r, err := wsutil.NextReader(c.conn, ws.StateServerSide)
	if err != nil {
		return nil, err
	}
	if h.OpCode.IsControl() {
		return nil, wsutil.ControlFrameHandler(c.conn, ws.StateServerSide)(h, r)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *WSClient) internalWrite(p []byte) error {
	c.io.Lock()
	defer c.io.Unlock()

	c.conn.(net.Conn).SetDeadline(time.Now().Add(WS_DEADLINE_DURATION_WRITE))

	_, err := c.conn.Write(p)
	return err
}

func (c *WSClient) WriteH(data echo.Map) {
	ret, err := json.Marshal(data)
	if err != nil {
		Log().Error().Err(err).Msg("Error when marshal data")
		return
	}
	c.Write(ret)
}

func (c *WSClient) Write(data []byte) {
	var buf bytes.Buffer
	w := wsutil.NewWriter(&buf, ws.StateServerSide, ws.OpText)
	w.Write(data)
	w.Flush()
	c.server.pool.Schedule(func() {
		c.internalWrite(buf.Bytes())
	})
}

func (c *WSClient) BroadcastInChannel(p []byte)  {
	c.io.RLock()
	if c.channel != nil {
		c.io.RUnlock()
		c.channel.Broadcast(p)
	}else{
		c.io.RUnlock()
	}
}

func (c *WSClient) BroadcastInChannelH(data echo.Map)  {
	c.io.RLock()
	if c.channel != nil {
		c.io.RUnlock()
		ret, err := json.Marshal(data)
		if err != nil {
			Log().Error().Err(err).Msg("Error when marshal data")
			return
		}
		c.channel.Broadcast(ret)
	}else{
		c.io.RUnlock()
	}
}

func (c *WSClient) OnAnyChannel() bool {
	c.io.RLock()
	defer c.io.RUnlock()
	return c.channel != nil
}

func (c *WSClient) LeaveChannel()  {
	c.io.RLock()
	defer c.io.RUnlock()
	if c.channel != nil {
		c.channel.Remove(c.id)
		c.channel = nil
	}
}
//-------------------------------------------------------------
// PRIVATE FUNCTIONS
//-------------------------------------------------------------
type WSChannel struct {
	name 						string
	server 						*AppWebSocket

	clientLock  				sync.RWMutex
	clients 					[]*WSClient
	mapClients 					map[int]*WSClient
	seq							int
	// channel for broadcast
	out 						chan []byte
}

func NewChannel(server *AppWebSocket, name string) *WSChannel{
	instance := &WSChannel{}
	instance.name = name
	instance.server = server
	instance.mapClients = make(map[int]*WSClient)
	instance.out = make(chan []byte, 1)

	go instance.broadcaster()

	return instance
}

func (this*WSChannel) Name() string {
	return this.name
}

func (this*WSChannel) Users() int {
	this.clientLock.RLock()
	defer this.clientLock.RUnlock()
	return len(this.clients)
}

func (this*WSChannel) broadcaster() {
	for bts := range this.out {
		this.clientLock.RLock()
		for _, u := range this.mapClients {
			u := u // For closure.
			this.server.pool.Schedule(func() {
				u.internalWrite(bts)
			})
		}
		this.clientLock.RUnlock()
	}
}

func (this*WSChannel) Close() {
	this.clientLock.Lock()
	for _, u := range this.clients {
		u.io.Lock()
		u.channel = nil
		u.io.Unlock()
	}
	this.clientLock.Unlock()
	close(this.out)
}

func (this*WSChannel) Broadcast(data []byte) {
	var buf bytes.Buffer
	w := wsutil.NewWriter(&buf, ws.StateServerSide, ws.OpText)
	w.Write(data)
	w.Flush()

	this.out <- buf.Bytes()
}


func (this*WSChannel) BroadcastH(data echo.Map) {
	var buf bytes.Buffer
	w := wsutil.NewWriter(&buf, ws.StateServerSide, ws.OpText)
	b, err := json.Marshal(data)
	if err != nil {
		Log().Error().Err(err).Msg("Encode error")
		return
	}
	w.Write(b)
	w.Flush()
	this.out <- buf.Bytes()
}


func (this*WSChannel) AddClient(client *WSClient) {
	// save client to map
	this.clientLock.Lock()
	{
		client.id = this.seq
		client.channel = this

		this.clients = append(this.clients, client)
		this.mapClients[client.id] = client

		this.seq++
	}
	this.clientLock.Unlock()

	Log().Info().Int("ID", client.id ).Str("Channel Name", this.name).Msg("Client added to channel")
}


func (this*WSChannel) Remove(id int) bool{
	this.clientLock.Lock()
	uuid, removed := this.internalRemove(id)
	this.clientLock.Unlock()
	if removed {
		this.server.OnLeaveChannel(uuid, this)
		Log().Info().Int("ID", id ).Str("Channel Name", this.name).Msg("Client leave Channel")
	}
	return removed
}

func (this*WSChannel) internalRemove(id int) (string, bool){
	if _, has := this.mapClients[id]; !has {
		return "", false
	}
	uuid :=  this.mapClients[id].uuid
	delete(this.mapClients, id)

	i := sort.Search(len(this.clients), func(i int) bool {
		return this.clients[i].id >= id
	})
	if i >= len(this.clients) {
		panic("chat: inconsistent state")
	}
	without := make([]*WSClient, len(this.clients)-1)
	copy(without[:i], this.clients[:i])
	copy(without[i:], this.clients[i+1:])
	this.clients = without

	return uuid, true
}

