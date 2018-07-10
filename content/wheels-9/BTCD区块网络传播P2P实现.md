btcd节点之间要组成一个P2P网络，又没有中心化节点，这些节点之间在P2P网络中进行信息传播，涉及到节点之间的连接管理、
地址管理、Peer节点的管理和Peer之间同步区块的协议等，相关代码和文件如下：

- btcd/peer：实现了P2P网络中Peer之间维持连接及收发wire协议消息的机制。
- btcd/wire：实现Bitcoin网络协议，定义理论Peers之间的协议消息，消息格式及包的封装和解析等。
- btcd/connmgr：管理Peer节点之间的TCP连接，包括监听本地端口等待其他节点连接和主动连接Peer节点等。
- btcd/addrmgr：收集、存储Bitcoin网络上的节点的地址，随机从地址集中选择路由可达的地址建立Peer连接，这些地址包括IPv4,IPv6及洋葱地址（onion address）等。
- btcd/server.go：btcd节点启动后执行的主要入口，定义了server及serverPeer类型，负责初始化和启动connmgr，addrmgr，以及响应Peer的协议消息等。

模块之间进行通信的关系如下图：（感谢oceanken的图）

![](http://chuantu.biz/t6/328/1528884979x-1376440132.png)

接下来，我们一个个分析其中的源码。首先了解P2P网络是如何组网，然后再进一步了解Bitcoin网络协议的实现。btcd/peer是实现Bitcoin P2P网络的核心模块。

### btcd/peer

- peer.go:包含了几乎全部的Peer相关逻辑实现代码；
- mruinvmap.go: 实现了一个简易的缓存Inverntory的LRU Cache。Inventory是节点向Peer节点回复或者通知的己方已经知道的Tx或者Block，它通过inv协议消息以Inventory Vectors的形式发往Peer节点。
- mrunoncemap.go:实现了一个缓存nonce值的LRU Cache。这里的nonce值是一个64位随机整数值，用于填充Peer间握手交换Version信息的nonce字段，nonce字段用来判断欲连接的Peer是否就是自己：节点发出Version消息时，会填入一个随机的nonce值，连接不同Peer节点时，节点会缓存所有为Version消息生成的nonce值；如果节点收到了一个Version消息且其中的nonce值是自己缓存的nonce中的一个，那么可以判断这个Version消息由自己发送给自己了;
- log.go: 提供一些log方法;

首先我们来看看peer.go里面有那些结构定义，Peer,Config和MessageListeners。Peer结构定义了Peer相关的属性和方法，Config结构
定义了与Peer相关的配置，MessageListeners定义了响应Peer消息的回调函数。

    type Peer struct {
    	// The following variables must only be used atomically.
    	bytesReceived uint64
    	bytesSent     uint64
    	lastRecv      int64
    	lastSend      int64
    	connected     int32
    	disconnect    int32

    	conn net.Conn

    	// 这些字段在创建时设置并且从不修改，因此它们可安全地在没有互斥锁的情况下同时读取。
    	addr    string
    	cfg     Config
    	inbound bool

    	flagsMtx             sync.Mutex // protects the peer flags below
    	na                   *wire.NetAddress
    	id                   int32
    	userAgent            string
    	services             wire.ServiceFlag
    	versionKnown         bool
    	advertisedProtoVer   uint32 // protocol version advertised by remote
    	protocolVersion      uint32 // negotiated protocol version
    	sendHeadersPreferred bool   // peer sent a sendheaders message
    	verAckReceived       bool
    	witnessEnabled       bool

    	wireEncoding wire.MessageEncoding

    	knownInventory     *mruInventoryMap
    	prevGetBlocksMtx   sync.Mutex
    	prevGetBlocksBegin *chainhash.Hash
    	prevGetBlocksStop  *chainhash.Hash
    	prevGetHdrsMtx     sync.Mutex
    	prevGetHdrsBegin   *chainhash.Hash
    	prevGetHdrsStop    *chainhash.Hash

    	// These fields keep track of statistics for the peer and are protected by the statsMtx mutex.
    	statsMtx           sync.RWMutex
    	timeOffset         int64
    	timeConnected      time.Time
    	startingHeight     int32
    	lastBlock          int32
    	lastAnnouncedBlock *chainhash.Hash
    	lastPingNonce      uint64    // Set to nonce if we have a pending ping.
    	lastPingTime       time.Time // Time we sent last ping.
    	lastPingMicros     int64     // Time for last ping to return.

    	stallControl  chan stallControlMsg
    	outputQueue   chan outMsg
    	sendQueue     chan outMsg
    	sendDoneQueue chan struct{}
    	outputInvChan chan *wire.InvVect
    	inQuit        chan struct{}
    	queueQuit     chan struct{}
    	outQuit       chan struct{}
    	quit          chan struct{}
    }

    type Config struct {
    	// NewestBlock指定一个回调方法，此方法能根据需求从其他节点提供最新的区块细节。
    	// 当然可能请求到一个块高度为0的区块，返回的是nil，然而大部分好的情况是返回准确的区块细节。
    	NewestBlock HashFunc
    	// HostToNetAddress方法给指定的host返回网络地址NetAddress，这个地址可能是nil，大部分情况会解析成一个IP地址
    	HostToNetAddress HostToNetAddrFunc
    	// Proxy表面连接用到了代理。这样做的唯一后果是防止泄漏tor代理地址，也就是说只有使用tor代理时才指定Proxy。
    	Proxy string
    	// 为版本发行指定用户代理名称。我们强烈建议指定这个值。
    	UserAgentName string
    	// 为版本发行指定版本号。我们强烈建议指定这个值，遵循 "major.minor.revision" e.g. "2.6.41" 。
    	UserAgentVersion string
    	// 为版本发行指定评论语，但不能使用非法的字符在里面，比如  '/', ':', '(', ')'.
    	UserAgentComments []string
    	// ChainParams，链参数，主要指定通信的节点要在哪条链上沟通以及如何沟通。如果忽略掉这个参数，测试网络将会被使用。
    	ChainParams *chaincfg.Params
    	// 指定由本地节点提供给发行版本哪个服务。如果忽略掉这个参数，将是0，因此没有本地节点提供服务。
    	Services wire.ServiceFlag
    	// 指定发行版本的最大协议版本。如果忽略这个参数，将使用peer.MaxProtocalVersion的值。
    	ProtocolVersion uint32
    	// 指定远程节点应当被告知不要为了transactions发送inv meesages。
    	DisableRelayTx bool
    	// 接收到节点的消息时，启动回调函数
    	Listeners MessageListeners
    }

    type MessageListeners struct {
    	// 当收到一个getaddr的比特币消息时启动此方法
    	OnGetAddr func(p *Peer, msg *wire.MsgGetAddr)
    	// 当收到一个addr的比特币消息时启动该方法
    	OnAddr func(p *Peer, msg *wire.MsgAddr)
    	// 当一个节点收到一个ping的比特币消息时启动该方法
    	OnPing func(p *Peer, msg *wire.MsgPing)
    	// 当一个节点收到一个pong的比特币消息时启动该方法
    	OnPong func(p *Peer, msg *wire.MsgPong)
    	// 当一个节点收到一个alert的比特币消息时启动该方法
    	OnAlert func(p *Peer, msg *wire.MsgAlert)
    	// 当一个节点收到一个mempool的比特币消息时启动该方法
    	OnMemPool func(p *Peer, msg *wire.MsgMemPool)
    	// 当一个节点收到一个tx的比特币消息时启动该方法
    	OnTx func(p *Peer, msg *wire.MsgTx)
    	// 当一个节点收到一个block的比特币消息时启动该方法
    	OnBlock func(p *Peer, msg *wire.MsgBlock, buf []byte)
    	// 当一个节点收到一个cfilter的比特币消息时启动该方法
    	OnCFilter func(p *Peer, msg *wire.MsgCFilter)
    	// 当一个节点收到一个cfheaders的比特币消息时启动该方法
    	OnCFHeaders func(p *Peer, msg *wire.MsgCFHeaders)
    	// 当一个节点收到一个cfcheckpt的比特币消息时启动该方法
    	OnCFCheckpt func(p *Peer, msg *wire.MsgCFCheckpt)
    	// 当一个节点收到一个inv的比特币消息时启动该方法
    	OnInv func(p *Peer, msg *wire.MsgInv)
    	// 当一个节点收到一个headers的比特币消息时启动该方法
    	OnHeaders func(p *Peer, msg *wire.MsgHeaders)
    	// 当一个节点收到一个notfound的比特币消息时启动该方法
    	OnNotFound func(p *Peer, msg *wire.MsgNotFound)
    	// 当一个节点收到一个getdata的比特币消息时启动该方法
    	OnGetData func(p *Peer, msg *wire.MsgGetData)
    	// 当一个节点收到一个getblocks的比特币消息时启动该方法
    	OnGetBlocks func(p *Peer, msg *wire.MsgGetBlocks)
    	// 当一个节点收到一个getheaders的比特币消息时启动该方法
    	OnGetHeaders func(p *Peer, msg *wire.MsgGetHeaders)
    	// 当一个节点收到一个getcfilters的比特币消息时启动该方法
    	OnGetCFilters func(p *Peer, msg *wire.MsgGetCFilters)
    	// 当一个节点收到一个getcfheaders的比特币消息时启动该方法
    	OnGetCFHeaders func(p *Peer, msg *wire.MsgGetCFHeaders)
    	// 当一个节点收到一个getcfcheckpt的比特币消息时启动该方法
    	OnGetCFCheckpt func(p *Peer, msg *wire.MsgGetCFCheckpt)
    	// 当一个节点收到一个feefilter的比特币消息时启动该方法
    	OnFeeFilter func(p *Peer, msg *wire.MsgFeeFilter)
    	// 当一个节点收到一个filteradd的比特币消息时启动该方法
    	OnFilterAdd func(p *Peer, msg *wire.MsgFilterAdd)
    	// 当一个节点收到一个filterclear的比特币消息时启动该方法
    	OnFilterClear func(p *Peer, msg *wire.MsgFilterClear)
    	// 当一个节点收到一个filterload的比特币消息时启动该方法
    	OnFilterLoad func(p *Peer, msg *wire.MsgFilterLoad)
    	// 当一个节点收到一个merkleblock的比特币消息时启动该方法
    	OnMerkleBlock func(p *Peer, msg *wire.MsgMerkleBlock)
    	// 当一个节点收到一个version的比特币消息时启动该方法
    	OnVersion func(p *Peer, msg *wire.MsgVersion)
    	// 当一个节点收到一个verack的比特币消息时启动该方法
    	OnVerAck func(p *Peer, msg *wire.MsgVerAck)
    	// 当一个节点收到一个reject的比特币消息时启动该方法
    	OnReject func(p *Peer, msg *wire.MsgReject)
    	// 当一个节点收到一个sendheaders的比特币消息时启动该方法
    	OnSendHeaders func(p *Peer, msg *wire.MsgSendHeaders)

    	// 当一个节点收到一个比特币消息时，就会吊起这个方法。
    	// 其参数由节点，读取的字节数，消息以及读取中的错误组成。
    	OnRead func(p *Peer, bytesRead int, msg wire.Message, err error)

    	// 当需要写入一个比特币消息到一个节点时，这个方法会被调用。
    	// 其参数由节点，写入的字节数，消息以及是否发生写入错误组成。
    	OnWrite func(p *Peer, bytesWritten int, msg wire.Message, err error)
    }

三个基础结构已经有了，然后从创建Peer对象的方法newPeerBase()方法来分析Peer:

    func newPeerBase(origCfg *Config, inbound bool) *Peer {
    	// 如果不是由调用者指定，则默认为最大的支持协议版本。
    	cfg := *origCfg // Copy to avoid mutating caller.
    	if cfg.ProtocolVersion == 0 {
    		cfg.ProtocolVersion = MaxProtocolVersion
    	}

    	// Set the chain parameters to testnet if the caller did not specify any.
    	if cfg.ChainParams == nil {
    		cfg.ChainParams = &chaincfg.TestNet3Params
    	}

    	p := Peer{
    		inbound:         inbound,
    		wireEncoding:    wire.BaseEncoding,
    		knownInventory:  newMruInventoryMap(maxKnownInventory),
    		stallControl:    make(chan stallControlMsg, 1), // nonblocking sync
    		outputQueue:     make(chan outMsg, outputBufferSize),
    		sendQueue:       make(chan outMsg, 1),   // nonblocking sync
    		sendDoneQueue:   make(chan struct{}, 1), // nonblocking sync
    		outputInvChan:   make(chan *wire.InvVect, outputBufferSize),
    		inQuit:          make(chan struct{}),
    		queueQuit:       make(chan struct{}),
    		outQuit:         make(chan struct{}),
    		quit:            make(chan struct{}),
    		cfg:             cfg, // Copy so caller can't mutate.
    		services:        cfg.Services,
    		protocolVersion: cfg.ProtocolVersion,
    	}
    	return &p
    }

创建Peer结构的属性非常多，关键属性包括：

- inbound：说明Peer是inbound还是outbound，如果当前节点主动连接Peer，则Peer为OutbandPeer；如果Peer主动连接当前节点，则Peer为InboundPeer。
- knownInventory：已经发送给Peer的Inventory的缓存。
- stallControl：带缓冲的stallControlMsg chan，在收，发消息的goroutine和超时控制goroutine之间通信。
- outputQueue：带缓冲的outMsg chan，实现了一个发送队列。
- sendQueue：缓冲大小为1的outMsg chan，用于将outputQueue中的outMsg按加入发送队列的顺序发送给Peer。
- sendDoneQueue：带缓冲的channel，用于通知维护发送队列的goroutine上一个消息已经发送完成，应该取下一条消息发送。
- outputInvChan：实现发送inv消息的发送队列，该队列以10s为周期向Peer发送inv消息。
- inQuit：用于通知收消息的goroutine已经退出。
- outQuit：用于通知发消息的goroutine已经退出，当收、发消息的goroutine均退出时，超时控制goroutine也将退出。
- quit：用于通知所有处理事务的goroutine退出。
- cfg：与Peer相关的Config，其中比较重要是Config中的MessageListeners，指明了处理与Peer收到的消息的响应函数。
- serrvices：用于记录Peer支持的服务，如SFNodeNetwork表明Peer是一个全节点，SFNodeGetUTXO表明Peer支持getutxos和utxos命令，SFNodeBloom表明Peer支持Bloom过滤。
- protocolVersion：用于记录Peer所用的协议版本。

这么多属性，很多与Peer实现的消息收发机制有关系，当然就会涉及到golang的channel和goroutine的概念，当然什么也不能阻挡我们造轮子。
这是我本人的学习笔记[golang的goroutine和channel的简单理解](https://www.jianshu.com/p/365209505661)。golang从语言级别就支持并发编程。golang的经典宣传语：

> 通过信道来共享内存，而不用通过共享内存来进行通信。

要处理接收信息和发送信息，就要对Peer进行启动，查找Peer的start()方法，找到如下代码：

    func (p *Peer) start() error {
    	log.Tracef("Starting peer %s", p)

    	negotiateErr := make(chan error)
    	go func() {
    		if p.inbound {
    			negotiateErr <- p.negotiateInboundProtocol()
    		} else {
    			negotiateErr <- p.negotiateOutboundProtocol()
    		}
    	}()

    	// Negotiate the protocol within the specified negotiateTimeout.
    	select {
    	case err := <-negotiateErr:
    		if err != nil {
    			return err
    		}
    	case <-time.After(negotiateTimeout):
    		return errors.New("protocol negotiation timeout")
    	}
    	log.Debugf("Connected to %s", p.Addr())

    	// The protocol has been negotiated successfully so start processing input
    	// and output messages.
    	go p.stallHandler()
    	go p.inHandler()
    	go p.queueHandler()
    	go p.outHandler()
    	go p.pingHandler()

    	// Send our verack message now that the IO processing machinery has started.
    	p.QueueMessage(wire.NewMsgVerAck(), nil)
    	return nil
    }

如果熟悉了前面的goroutine和信道知识点，看这段代码就不难了。主要包含了下面的步骤：

- 1.一开始就起一个goroutine，用来与Peer的交换Version消息，negotiateInboundProtocol()和negotiateOutboundProtocol()都是为了交换Peer的版本信息
- 2.如果Version握手失败或者超时，则返回错误，Peer建立连接关系失败。
- 3.如果握手成功，则启动5个新的goroutine来收发消息。
    -  go p.stallHandler()：处理消息超时  handles stall detection for the peer
    -  go p.inHandler()：接收Peer消息 handles all incoming messages for the peer
    -  go p.queueHandler()：维护消息发送队列 handles the queuing of outgoing data for the peer
    -  go p.outHandler()：向Peer发送消息 handles all outgoing messages for the peer
    -  go p.pingHandler()：向Peer周期性发送心跳 periodically pings the peer
- 4.最后，向Peer发送verack消息，双方完成握手。

Peer 的start()成功后，节点间的Peer关系便成功建立，可以进一步交换其他协议消息。多个节点之间互相建立Peer连接关系，
这样就形成了一张P2P的网络。接下来我们深入Peer之间如何通过握手建立连接和实现接受发送消息的。

##### Peer之间握手

Peer节点要start()，首先就是要交换Version消息，用到的方法是negotiateInboundProtocol()和和negotiateOutboundProtocol()

    // 等待从Peer接收到的版本消息，然后发送我们的版本消息。如果事件没有按照该顺序发生，那么它将返回一个错误。
    func (p *Peer) negotiateInboundProtocol() error {
    	if err := p.readRemoteVersionMsg(); err != nil {
    		return err
    	}
    	return p.writeLocalVersionMsg()
    }

分为两步：

- 等待并读取，处理Peer发过来的Version消息
- 向Peer发送自己的Version消息

readRemoteVersionMsg()方法中首先是读取消息，然后是处理消息，我们深入看一下处理消息的代码：

    // 在Peer接收远程Peer的一个Version消息时，此方法就会被调用。当然，如果远程Peer的版本与此节点不兼容，就返回错误。
    func (p *Peer) handleRemoteVersionMsg(msg *wire.MsgVersion) error {
    	// Detect self connections.
    	if !allowSelfConns && sentNonces.Exists(msg.Nonce) {
    		return errors.New("disconnecting peer connected to self")
    	}

    	if uint32(msg.ProtocolVersion) < minAcceptableProtocolVersion {
    		reason := fmt.Sprintf("protocol version must be %d or greater",
    			minAcceptableProtocolVersion)
    		return errors.New(reason)
    	}

    	// Updating a bunch of stats including block based stats, and the peer's time offset.
    	p.statsMtx.Lock()
    	p.lastBlock = msg.LastBlock
    	p.startingHeight = msg.LastBlock
    	p.timeOffset = msg.Timestamp.Unix() - time.Now().Unix()
    	p.statsMtx.Unlock()

    	// Negotiate the protocol version.
    	p.flagsMtx.Lock()
    	p.advertisedProtoVer = uint32(msg.ProtocolVersion)
    	p.protocolVersion = minUint32(p.protocolVersion, p.advertisedProtoVer)
    	p.versionKnown = true
    	log.Debugf("Negotiated protocol version %d for peer %s",
    		p.protocolVersion, p)

    	// Set the peer's ID.
    	p.id = atomic.AddInt32(&nodeCount, 1)

    	// Set the supported services for the peer to what the remote peer advertised.
    	p.services = msg.Services

    	// Set the remote peer's user agent.
    	p.userAgent = msg.UserAgent

    	// Determine if the peer would like to receive witness data with transactions, or not.
    	if p.services&wire.SFNodeWitness == wire.SFNodeWitness {
    		p.witnessEnabled = true
    	}
    	p.flagsMtx.Unlock()

    	if p.services&wire.SFNodeWitness == wire.SFNodeWitness {
    		p.wireEncoding = wire.WitnessEncoding
    	}

    	return nil
    }

Peer在处理Version消息时，主要逻辑：

- 1 !allowSelfConns && sentNonces.Exists(msg.Nonce) 判断是否允许自己连接自己，检测Version消息里的Nonce是否是缓存的nonce值。根据此判断Version消息是否由自己发给自己。
- 2 检测Version消息里的ProtocalVersion，如果Peer的协议版本低于minAcceptableProtocolVersion，则拒绝连接。
- 3 开始更新Peer的相关信息，如Peer的最新区块高度、Peer与本地节点的时间偏移等。
- 4 更新Peer的版本号，支持的服务，UserAgent等信息，同时为其分配一个id。

Peer的区块高度，支持的服务等消息将用于本地节点与远程节点同步Peer的区块。为了后续的消息交换和区块同步，第一步进行Version
消息的交换是必须的。那就会涉及到Peer发送Version消息和接收Version消息。

- 填充和封装Version消息，将是wire来负责，下面将会讲到
- 在Peer的start()方法中，起了5个goroutine，第一个用来握手，后面各个handler处理channel的消息。

首先看看inHandler()：

    func (p *Peer) inHandler() {
    	// 当接收到一条新消息并在处理后，计时器将停止重置
    	idleTimer := time.AfterFunc(idleTimeout, func() {
    		log.Warnf("Peer %s no answer for %s -- disconnecting", p, idleTimeout)
    		p.Disconnect()
    	})

    out:
    	for atomic.LoadInt32(&p.disconnect) == 0 {
    		// 读取一条消息并在读取完成后立即停止空闲计时器。如果需要，计时器将在下一个迭代中重新设置。
    		rmsg, buf, err := p.readMessage(p.wireEncoding)
    		idleTimer.Stop()
    		if err != nil {
    			// 为了让回归测试使用错误的消息，当我们处于回归测试模式时，不要断开Peer，错误是允许的错误之一。
    			if p.isAllowedReadError(err) {
    				log.Errorf("Allowed test error from %s: %v", p, err)
    				idleTimer.Reset(idleTimeout)
    				continue
    			}

    			// 如果本地Peer没有被强制断开，而远程Peer没有断开连接，那么只记录错误并发送拒绝消息。
    			if p.shouldHandleReadError(err) {
    				errMsg := fmt.Sprintf("Can't read message from %s: %v", p, err)
    				if err != io.ErrUnexpectedEOF {
    					log.Errorf(errMsg)
    				}


    				// 发送一个malformed的拒绝消息，然后在断开连接之前等待消息被送达

    				// NOTE: 理想情况下，如果至少有那么多的消息是有效的，那么这将包括在头部的命令，但是目前还没有通过电线公开，所以只是为命令使用了错误。
    				p.PushRejectMsg("malformed", wire.RejectMalformed, errMsg, nil,
    					true)
    			}
    			break out
    		}
    		atomic.StoreInt64(&p.lastRecv, time.Now().Unix())
    		p.stallControl <- stallControlMsg{sccReceiveMessage, rmsg}

    		// Handle each supported message type.
    		p.stallControl <- stallControlMsg{sccHandlerStart, rmsg}
    		switch msg := rmsg.(type) {
    		case *wire.MsgVersion:

    			p.PushRejectMsg(msg.Command(), wire.RejectDuplicate,
    				"duplicate version message", nil, true)
    			break out

    		case *wire.MsgVerAck:

    			// No read lock is necessary because verAckReceived is not written
    			// to in any other goroutine.
    			if p.verAckReceived {
    				log.Infof("Already received 'verack' from peer %v -- "+
    					"disconnecting", p)
    				break out
    			}
    			p.flagsMtx.Lock()
    			p.verAckReceived = true
    			p.flagsMtx.Unlock()
    			if p.cfg.Listeners.OnVerAck != nil {
    				p.cfg.Listeners.OnVerAck(p, msg)
    			}

    		case *wire.MsgGetAddr:
    			if p.cfg.Listeners.OnGetAddr != nil {
    				p.cfg.Listeners.OnGetAddr(p, msg)
    			}

    		case *wire.MsgAddr:
    			if p.cfg.Listeners.OnAddr != nil {
    				p.cfg.Listeners.OnAddr(p, msg)
    			}

    		......

    		default:
    			log.Debugf("Received unhandled message of type %v "+
    				"from %v", rmsg.Command(), p)
    		}
    		p.stallControl <- stallControlMsg{sccHandlerDone, rmsg}

    		// A message was received so reset the idle timer.
    		idleTimer.Reset(idleTimeout)
    	}

    	// Ensure the idle timer is stopped to avoid leaking the resource.
    	idleTimer.Stop()

    	// Ensure connection is closed.
    	p.Disconnect()

    	close(p.inQuit)
    	log.Tracef("Peer input handler done for %s", p)
    }

其主要步骤有：

- 1.设定一个idleTimer，当接收到一条新消息并在处理后，计时器将停止重置，当然其也设定了超时时间，设定为5分钟。
    - 每隔5分钟进行检测，如果没有从Peer接收到心跳消息，就主动与该Peer断开连接。当然节点会主动向其他Peer发送心跳消息。
- 2.循环读取和处理从Peer发送过来的消息。rmsg, buf, err := p.readMessage(p.wireEncoding)。只要收到消息，idleTimer就暂停。
    - p.stallControl <- stallControlMsg{sccReceiveMessage, rmsg} inHandler向stallhandler发送sccReceiverMessage消息。
    - p.stallControl <- stallControlMsg{sccHandlerStart, rmsg}  inHandler向stallHandler发送sccHandlerStart消息，stallHandler会根据消息来计算节点接收并处理所消耗的时间。
- 3.在处理Peer发送过来的消息时，inHandler可能先对其做处理，比如MsgPing和MsgPong，也可能不做任何处理，如MsgBlock等，然后回调MessageListener的对应函数做处理。
- 4.在处理完一条消息后，inhandler向stallHandler发送sccHandlerDone，通知stallHandler消息处理完毕。同时将idleTimer复位再次开始计时并等待读取下一条消息。
    - p.stallControl <- stallControlMsg{sccHandlerDone, rmsg}
- 5.当主动调用Disconnect()与Peer断开连接后，消息读取和处理循环将退出，inHandler协程也准备退出。退出之前，
    - 先将idleTimer停止，idleTimer.Stop()
    - 强制与Peer断开连接，p.Disconnect()
    - 向stallhandler通知自己已经退出，close(p.inQuit)

inHandler协程主要处理接收消息，并回调MessageListener中的消息处理函数对消息进行处理，需要函数处理消息时，不能超过太久，不然耗时太久引起超时断连。

outHandler主要负责发送消息：

    func (p *Peer) outHandler() {
    out:
    	for {
    		select {
    		case msg := <-p.sendQueue:
    			switch m := msg.msg.(type) {
    			case *wire.MsgPing:
    				// 只期望在以后的协议版本中有一个pong消息。
    				if p.ProtocolVersion() > wire.BIP0031Version {
    					p.statsMtx.Lock()
    					p.lastPingNonce = m.Nonce
    					p.lastPingTime = time.Now()
    					p.statsMtx.Unlock()
    				}
    			}

    			p.stallControl <- stallControlMsg{sccSendMessage, msg.msg}

    			err := p.writeMessage(msg.msg, msg.encoding)
    			if err != nil {
    				p.Disconnect()
    				if p.shouldLogWriteError(err) {
    					log.Errorf("Failed to send message to "+
    						"%s: %v", p, err)
    				}
    				if msg.doneChan != nil {
    					msg.doneChan <- struct{}{}
    				}
    				continue
    			}

    			// 在这一点上，消息被成功地发送了，所以更新最后一次发送时间，发送它已经发送的消息的发送者（如果请求的话），并将发送队列发送给发送下一个排队的消息。
    			atomic.StoreInt64(&p.lastSend, time.Now().Unix())
    			if msg.doneChan != nil {
    				msg.doneChan <- struct{}{}
    			}
    			p.sendDoneQueue <- struct{}{}

    		case <-p.quit:
    			break out
    		}
    	}

    	<-p.queueQuit

    	// 在我们离开之前，先把所有的等待通道都排掉，这样我们就不会留下什么东西等着我们。我们已经等待了queueQuit，因此我们可以确信，我们不会错过发送到sendQueue的任何东西。
    cleanup:
    	for {
    		select {
    		case msg := <-p.sendQueue:
    			if msg.doneChan != nil {
    				msg.doneChan <- struct{}{}
    			}
    			// no need to send on sendDoneQueue since queueHandler
    			// has been waited on and already exited.
    		default:
    			break cleanup
    		}
    	}
    	close(p.outQuit)
    	log.Tracef("Peer output handler done for %s", p)
    }

outHandler主要是从sendQueue循环取出消息，并调用writeMesssage()向Peer发送消息。
    - case msg := <-p.sendQueue:
    - err := p.writeMessage(msg.msg, msg.encoding)

当消息发送前，它向stallHandler发送sccSendMessage消息，通知stallHandler开始跟踪这条消息的响应是否超时。
    - p.stallControl <- stallControlMsg{sccSendMessage, msg.msg}

通知stallHandler开始跟踪这条消息的响应是否超时，消息发成功后，通过sendDoneQueue channel通知queueHandler发送下一条消息。
    - p.sendDoneQueue <- struct{}{}

sendQueue是buffer size为1 的channel，与sendDoneQueue配合保证发送缓冲队列outputQueue里的消息按顺序一一发送。
    - case msg := <-p.sendQueue:

当Peer断开连接时，p.quit的接收代码会被触发，从而让循环退出。

通过queeuQuit同步，outHandler退出之前需要等待queueHandler退出，是为了让queueHandler将发送缓冲中的消息清空。最后通过outQuit channel通知stallHandler自己退出。
    - close(p.outQuit)

发送消息的队列由queueHandler维护，它通过sendQueue将队列中的消息送往outHandler并向Peer发送。queueHandler还专门处理了Inventory的发送：

    func (p *Peer) queueHandler() {
    	pendingMsgs := list.New()
    	invSendQueue := list.New()
    	trickleTicker := time.NewTicker(trickleTimeout)
    	defer trickleTicker.Stop()

    	// 我们保留等待标志，以便我们知道是否有消息排队等待outHandler。
    	// 我们可以使用列表的头部来表示这个问题，但是我们对它是否已经在清理时间得到了相当的关注，
        // 并且因此谁发送了消息的完成通道。为了避免这种混淆，我们保留了一个不同的标志，pendingMsgs只包含了我们尚未传递给outHandler的消息。
    	waiting := false

    	// To avoid duplication below.
    	queuePacket := func(msg outMsg, list *list.List, waiting bool) bool {
    		if !waiting {
    			p.sendQueue <- msg
    		} else {
    			list.PushBack(msg)
    		}
    		// we are always waiting now.
    		return true
    	}
    out:
    	for {
    		select {
    		case msg := <-p.outputQueue:
    			waiting = queuePacket(msg, pendingMsgs, waiting)

    		// 通过网络套接字发送消息时会通知此channel。
    		case <-p.sendDoneQueue:
    			// 如果待处理消息队列中没有更多消息，则不再等待。
    			next := pendingMsgs.Front()
    			if next == nil {
    				waiting = false
    				continue
    			}

    			// 通知outHandler关于下一个要异步发送的项目。
    			val := pendingMsgs.Remove(next)
    			p.sendQueue <- val.(outMsg)

    		case iv := <-p.outputInvChan:
    			// No handshake?  They'll find out soon enough.
    			if p.VersionKnown() {
    				// 如果这是一个新的区块，那么我们会立即爆出它，剥除inv队列。
    				if iv.Type == wire.InvTypeBlock ||
    					iv.Type == wire.InvTypeWitnessBlock {

    					invMsg := wire.NewMsgInvSizeHint(1)
    					invMsg.AddInvVect(iv)
    					waiting = queuePacket(outMsg{msg: invMsg},
    						pendingMsgs, waiting)
    				} else {
    					invSendQueue.PushBack(iv)
    				}
    			}

    		case <-trickleTicker.C:
    			// 如果我们断开连接或不存在排队的库存，则不要发送任何内容。如果发送队列有任何条目，则会知道版本。
    			if atomic.LoadInt32(&p.disconnect) != 0 ||
    				invSendQueue.Len() == 0 {
    				continue
    			}

    			// 根据需要创建和发送尽可能多的inv消息以清空库存发送队列。
    			invMsg := wire.NewMsgInvSizeHint(uint(invSendQueue.Len()))
    			for e := invSendQueue.Front(); e != nil; e = invSendQueue.Front() {
    				iv := invSendQueue.Remove(e).(*wire.InvVect)

    				// 在初始检查后不要发送已知的库存。
    				if p.knownInventory.Exists(iv) {
    					continue
    				}

    				invMsg.AddInvVect(iv)
    				if len(invMsg.InvList) >= maxInvTrickleSize {
    					waiting = queuePacket(
    						outMsg{msg: invMsg},
    						pendingMsgs, waiting)
    					invMsg = wire.NewMsgInvSizeHint(uint(invSendQueue.Len()))
    				}

    				// 将正在中继的inventory添加到Peer的已知inventory中。
    				p.AddKnownInventory(iv)
    			}
    			if len(invMsg.InvList) > 0 {
    				waiting = queuePacket(outMsg{msg: invMsg},
    					pendingMsgs, waiting)
    			}

    		case <-p.quit:
    			break out
    		}
    	}

    	// 在我们离开之前排空任何等待频道，所以我们不会留下任何等待我们的东西。
    	for e := pendingMsgs.Front(); e != nil; e = pendingMsgs.Front() {
    		val := pendingMsgs.Remove(e)
    		msg := val.(outMsg)
    		if msg.doneChan != nil {
    			msg.doneChan <- struct{}{}
    		}
    	}
    cleanup:
    	for {
    		select {
    		case msg := <-p.outputQueue:
    			if msg.doneChan != nil {
    				msg.doneChan <- struct{}{}
    			}
    		case <-p.outputInvChan:
    			// Just drain channel
    		// sendDoneQueue is buffered so doesn't need draining.
    		default:
    			break cleanup
    		}
    	}
    	close(p.queueQuit)
    	log.Tracef("Peer queue handler done for %s", p)
    }

queueHandler()中的主要步骤：

- 1.定义一个函数值，功能是当从outputQueue接收到待发送消息时，如果有消息正在通过outHandler发送，则将消息缓存到pendingMsgs或invSendQueue。
    - queuePacket := func(msg outMsg, list *list.List, waiting bool)
- 2.循环处理channel消息。这里的select语句没有定义default分支，如果管道中没有数据时，循环将阻塞在select语句处。
    - for { select { case msg := <-p.outputQueue: ...
- 3.当有发送消息的请求时，发送方于是向outoutQueue写入数据，接收消息代码会被触发，并调用queuePacket()，要么立即向outHandler，要么缓存起来排队发送。
    - case msg := <-p.outputQueue:
- 4.当outHandler发送完一条消息时，它向sendDoneQueue写入数据，接收代码被触发，queueHandler从缓存在pendingMsgs中的待发送消息取出一条发往outHandler。
    - case <-p.sendDoneQueue:
    - next := pendingMsgs.Front()
    -  p.sendQueue <- val.(outMsg)
- 5.当要发送Inventory时，发送方要向outputInvChan写入数据，接收代码被触发，待发送的Inventory将会被缓存到invSendQueue中。
    - case iv := <-p.outputInvChan:
    - invSendQueue.PushBack(iv)
- 6.trickle Ticker 10s被触发一次，首先从invSendQueue中取出一条Inventory，随后验证是否已经向Peer发送过
    -  case <-trickleTicker.C:
    - 如果是新的Inventory，则将各个Inventory组成Inventory Vector，通过inv消息发往Peer。 if p.knownInventory.Exists(iv) { ...
    - Inventory Vector是有最大容量的。 if len(invMsg.InvList) >= maxInvTrickleSize { ...
    - 已经发送过的Inventory会缓存下来，防止重复发送。 p.AddKnownInventory(iv)
- 7.当调用Peer的Disconnect()时，p.quit的接收代码会被出发，循环退出。
    - 将pendingMsgs中的待发送消息清空  for e := pendingMsgs.Front(); e != nil; e = pendingMsgs.Front() { ...
    - 将管道中的消息清空   cleanup:  for { ...
    - 通过queueQuit channel通知outHandler退出    close(p.queueQuit)

总结：queueHandler()通过outputQueue和outputInvChan这两个带缓冲的channel，以及pendingMsgs和invSendQueue两个List，实现了发送消息队列。

如果保证消息按照顺序串行发送呢，是简单的通过缓存大小为1的channel sendQueue来保证的。

inhandler,outHandler和queueHandler在不同goroutine中执行，实现了异步接发消息。

当然，在inHandler中，消息的接收处理也是一条一条地串行处理的，如果没有超时控制，假设某一段时间内发送队列中有大量需要发送的消息，
而且inHandler中处理某些消息又消耗大量时间导致后续消息无法读取时，Peer之间的消息交换将发生严重的“拥塞”，所以stallhandler中超时处理是必须的。

    // stallHandler处理对等的失速检测。这需要跟踪预期的回应，并在考虑回调花费的时间的同时为其分配期限。
    func (p *Peer) stallHandler() {
    	// 这些变量用于在执行回调的时间内向前调整截止时间。
    	// 这是因为直到前一个消息完成处理（包括回调）才读取新消息，因此接收给定消息的响应的最后期限也必须考虑处理时间。
    	var handlerActive bool
    	var handlersStartTime time.Time
    	var deadlineOffset time.Duration

    	// 跟踪预期的响应截止时间。
    	pendingResponses := make(map[string]time.Time)

    	// stallTicker用于定期检查超出预计期限的待处理响应，并由于停止而断开Peer。
    	stallTicker := time.NewTicker(stallTickInterval)
    	defer stallTicker.Stop()

    	// 用于检测当输入和输出处理协程goroutine都完成。
    	var ioStopped bool
    out:
    	for {
    		select {
    		case msg := <-p.stallControl:
    			switch msg.command {
    			case sccSendMessage:
    				// 如果需要，为预期的响应消息添加最后期限
    				p.maybeAddDeadline(pendingResponses,
    					msg.message.Command())

    			case sccReceiveMessage:
    				// 从预期的响应地图中移除收到的消息。由于某些命令期望得到一组响应中的一个，因此应相应地删除预期组中的所有内容。
    				switch msgCmd := msg.message.Command(); msgCmd {
    				case wire.CmdBlock:
    					fallthrough
    				case wire.CmdMerkleBlock:
    					fallthrough
    				case wire.CmdTx:
    					fallthrough
    				case wire.CmdNotFound:
    					delete(pendingResponses, wire.CmdBlock)
    					delete(pendingResponses, wire.CmdMerkleBlock)
    					delete(pendingResponses, wire.CmdTx)
    					delete(pendingResponses, wire.CmdNotFound)

    				default:
    					delete(pendingResponses, msgCmd)
    				}

    			case sccHandlerStart:
    				// Warn on unbalanced callback signalling.
    				if handlerActive {
    					log.Warn("Received handler start " +
    						"control command while a " +
    						"handler is already active")
    					continue
    				}

    				handlerActive = true
    				handlersStartTime = time.Now()

    			case sccHandlerDone:
    				// Warn on unbalanced callback signalling.
    				if !handlerActive {
    					log.Warn("Received handler done " +
    						"control command when a " +
    						"handler is not already active")
    					continue
    				}

    				// 延长执行回调所需的活动期限
    				duration := time.Since(handlersStartTime)
    				deadlineOffset += duration
    				handlerActive = false

    			default:
    				log.Warnf("Unsupported message command %v",
    					msg.command)
    			}

    		case <-stallTicker.C:
    			// 根据处理程序自上次打勾以来执行的时间，计算应用于截止日期的偏移量
    			now := time.Now()
    			offset := deadlineOffset
    			if handlerActive {
    				offset += now.Sub(handlersStartTime)
    			}

    			// 如果有未决的回复没有按其调整的截止日期到达，则断开Peer。
    			for command, deadline := range pendingResponses {
    				if now.Before(deadline.Add(offset)) {
    					continue
    				}

    				log.Debugf("Peer %s appears to be stalled or "+
    					"misbehaving, %s timeout -- "+
    					"disconnecting", p, command)
    				p.Disconnect()
    				break
    			}

    			// Reset the deadline offset for the next tick.
    			deadlineOffset = 0

    		case <-p.inQuit:
    			// 一旦输入和输出处理程序都完成，失速检测处理程序就可以退出。
    			if ioStopped {
    				break out
    			}
    			ioStopped = true

    		case <-p.outQuit:
    			//  一旦输入和输出处理程序都完成，失速检测处理程序就可以退出。
    			if ioStopped {
    				break out
    			}
    			ioStopped = true
    		}
    	}

    	// 在离开之前排空任何等待通道，因此在该goroutine上没有任何等待。
    cleanup:
    	for {
    		select {
    		case <-p.stallControl:
    		default:
    			break cleanup
    		}
    	}
    	log.Tracef("Peer stall handler done for %s", p)
    }

其主要逻辑：

- 1.当收到outHandler发来的sccSendMessage时，将为已经发送的消息设定收到响应消息的超时时间deadline，并缓存入pendingResponses中。
    -  case sccSendMessage:
    -  p.maybeAddDeadline(pendingResponses,msg.message.Command())
- 2.当收到inHandler发来的sccReceiveMessage时，如果是响应消息，则将对应消息命令和其deadline从pendingResponses中移除，不需再跟踪该消息是否超时。
    -  case sccReceiveMessage:
    - default: delete(pendingResponses, msgCmd)
    - 为何只根据消息命令或者类型来匹配请求和响应，并没有通过序列号或请求ID来严格匹配？
        - Peer之间的发送和接收都是串行处理
        - Peer之间在节点同步到最新区块后，消息交换不是非常频繁
- 3.当收到inHandler发送来的sccHandlerStart时，说明inHandler开始处理接收到的消息。
    - case sccHandlerStart: ...
    - 这个机制就是为了防止下一条响应消息因为当前消息处理时间太长而超时
    - stallHandler收到scchandlerStart和sccHandlerDone时，会计算当前消息的处理时间消耗，并在检测下一条响应消息的时候，检测前一条消息的处理时间是否超时。
- 4.当收到inHandler发来的sccHandlerDone时，表明当前接收到的消息已经处理完毕，计算消耗时间deadlineOffset，这个时间差将被用于调节下一个响应消息的超时门限。
    - case sccHandlerDone: ...
- 5.stallTicker每隔15s触发，用于周期性地检查是否有消息的响应超时，如果有响应已经超时，则主动断开该Peer连接。
    - case <-stallTicker.C:
    - if handlerActive { offset += now.Sub(handlersStartTime) } 如果当前检测时点与上一个检查时点之间有一条接收消息在处理，则超时门限延长
    -  if now.Before(deadline.Add(offset)) { continue } 但接收消息处理太耗时，即便延长超时门限，但下一条消息之后的响应消息还会超时
- 6.当inHandler和outHandler均退出后，stallHandler才结束处理循环，准备退出。
    - case <-p.inQuit:
    - case <-p.outQuit:
- 7.stallHandler将stallControl channel中的消息清空，并最后退出。
    - cleanup: for { ...

总结：stallHandler跟踪发送消息与对应的响应消息，每隔15s检测是否有响应消息超时。
同时修正了当前响应消息处理时间对下一条响应消息超时检查的影响，当超时发生时主动断开与Peer的连接，重新选择其他Peer开始同步，保证了Peer接发消息
不会因为网络延迟或处理耗时而影响区块同步效率。

当然为了保证Peer之间的连接关系，是需要Peer之间有心跳检测的，Ping消息的发送由pingHandler来处理，Peer节点收到后回复Pong消息。

    func (p *Peer) pingHandler() {
    	pingTicker := time.NewTicker(pingInterval)
    	defer pingTicker.Stop()

    out:
    	for {
    		select {
    		case <-pingTicker.C:
    			nonce, err := wire.RandomUint64()
    			if err != nil {
    				log.Errorf("Not sending ping to %s: %v", p, err)
    				continue
    			}
    			p.QueueMessage(wire.NewMsgPing(nonce), nil)

    		case <-p.quit:
    			break out
    		}
    	}
    }

pingHandler的逻辑相对简单，主要以2分钟为周期向Peer发送Ping消息，当p.quit被关闭时，pingHandler退出。

peer.go的5个handler或goroutine的执行过程全部看完，他们就是Peer之间收发消息的大概框架，底层的消息的读写是readMessage和writeMessage方法。

    func (p *Peer) readMessage(encoding wire.MessageEncoding) (wire.Message, []byte, error) {
    	n, msg, buf, err := wire.ReadMessageWithEncodingN(p.conn,
    		p.ProtocolVersion(), p.cfg.ChainParams.Net, encoding)
    	atomic.AddUint64(&p.bytesReceived, uint64(n))
    	if p.cfg.Listeners.OnRead != nil {
    		p.cfg.Listeners.OnRead(p, n, msg, err)
    	}
    	if err != nil {
    		return nil, nil, err
    	}

    	// Use closures to log expensive operations so they are only run when
    	// the logging level requires it.
    	log.Debugf("%v", newLogClosure(func() string {
    		// Debug summary of message.
    		summary := messageSummary(msg)
    		if len(summary) > 0 {
    			summary = " (" + summary + ")"
    		}
    		return fmt.Sprintf("Received %v%s from %s",
    			msg.Command(), summary, p)
    	}))
    	log.Tracef("%v", newLogClosure(func() string {
    		return spew.Sdump(msg)
    	}))
    	log.Tracef("%v", newLogClosure(func() string {
    		return spew.Sdump(buf)
    	}))

    	return msg, buf, nil
    }

    func (p *Peer) writeMessage(msg wire.Message, enc wire.MessageEncoding) error {
    	// Don't do anything if we're disconnecting.
    	if atomic.LoadInt32(&p.disconnect) != 0 {
    		return nil
    	}

    	// Use closures to log expensive operations so they are only run when
    	// the logging level requires it.
    	log.Debugf("%v", newLogClosure(func() string {
    		// Debug summary of message.
    		summary := messageSummary(msg)
    		if len(summary) > 0 {
    			summary = " (" + summary + ")"
    		}
    		return fmt.Sprintf("Sending %v%s to %s", msg.Command(),
    			summary, p)
    	}))
    	log.Tracef("%v", newLogClosure(func() string {
    		return spew.Sdump(msg)
    	}))
    	log.Tracef("%v", newLogClosure(func() string {
    		var buf bytes.Buffer
    		_, err := wire.WriteMessageWithEncodingN(&buf, msg, p.ProtocolVersion(),
    			p.cfg.ChainParams.Net, enc)
    		if err != nil {
    			return err.Error()
    		}
    		return spew.Sdump(buf.Bytes())
    	}))

    	// Write the message to the peer.
    	n, err := wire.WriteMessageWithEncodingN(p.conn, msg,
    		p.ProtocolVersion(), p.cfg.ChainParams.Net, enc)
    	atomic.AddUint64(&p.bytesSent, uint64(n))
    	if p.cfg.Listeners.OnWrite != nil {
    		p.cfg.Listeners.OnWrite(p, n, msg, err)
    	}
    	return err
    }

显而易见，收发消息都跑到wire的message.go里面来了，ReadMessageWithEncodingN()和WriteMessageWithEncodingN()。

Peer收发消息的机制有了一个初步的了解，但Peer之间收发消息的一个前提是Peer之间建立TCP连接为前提的。

接下来我们分析一下Peer之间如何建立及维护TCP连接。节点之间可以直接建立连接，也可以通过代理（Proxy）连接。
甚至可以为了匿名性，通过洋葱路由代理（Onion Proxy）建立TCP连接。

#### btcd/connmgr

- connmanager.go:处理建立新的连接，通知连接状态，重连及断开连接等主要逻辑。
- dynamicbanscore.go:实现了一个动态记分器，用于记录Peer之间消息交换的频率，当分数大于设定的门限时会主动断开连接，可以防止类似DDoS攻击。
- seed.go:负责将内置于全节点客户端里的种子节点的地址解析成Bitcoin协议里定义的网络地址。
- tor.go:通过洋葱代理建立连接的节点，需要在Tor网络上的最后一跳，即退出节点上进行DNS解析，然后将解析结果通过洋葱代理返回给节点
    - 通过洋葱代理进行DNS解析的SOCKS消息交换。
    - DNS解析并不是解析洋葱地址，而是解析公网上的域名或者hostname
- log.go:提供logger初始化及设定logger等方法。

重点分析connmanager.go，其中主要的结构包括：ConnManager,Config和ConnReq:

    type ConnManager struct {
    	// The following variables must only be used atomically.
    	connReqCount uint64             // 记录主动连接其他节点的连接数量
    	start        int32              // 标识connmgr已经启动
    	stop         int32              // 标识connmgr已经结束
    	cfg            Config           // 设定相关的配置，在Config的定义中介绍
    	wg             sync.WaitGroup   // 用于同步connmgr的退出状态，调用方可以阻塞等待connmgr的工作协程退出
    	failedAttempts uint64           // 某个连接失败后，ConnMgr尝试选择新的Peer地址连接的总次数
    	requests       chan interface{} // 用于与connmgr工作协程通信的管道
    	quit           chan struct{}    // 用于通知工作协程退出
    }

ConnManager的定义就依赖Config：

    type Config struct {
        // 节点上所有等待外部连接的监听点
    	Listeners []net.Listener

    	// 节点答应并接受外部连接后的回调函数
    	OnAccept func(net.Conn)

    	// 节点主动向外连接Peer的最大个数
    	TargetOutbound uint32

        // 连接失败后发起重新连接的等待时间，默认为5s，默认的最大重连等待时间为5min
    	RetryDuration time.Duration

    	// 连接建立成功后的回调函数
    	OnConnection func(*ConnReq, net.Conn)

    	// 连接关闭后的回调函数
    	OnDisconnection func(*ConnReq)

    	// 连接失败后，ConnMgr可能会选择新的Peer进行连接，GetNewAddress函数提供获取新Peer地址的方法，
    	// 最终会调用addrManager的GetAddress()来分配新地址
    	GetNewAddress func() (net.Addr, error)

    	// 定义建立TCP连接的方式，是直接还是代理
    	Dial func(net.Addr) (net.Conn, error)
    }

ConnReq描述了一个连接：

    type ConnReq struct {
    	// The following variables must only be used atomically.
    	id uint64               // 连接的序号，用于索引
    	Addr      net.Addr      // 连接的目的地址
    	Permanent bool          // 标记是否与Peer保持永久连接
    	conn       net.Conn     // 连接成功后，真实的net.Conn对象
    	state      ConnState    // 连接的状态，有ConnPending，ConnEstablished，ConnDisconnected及ConnFailed等
    	stateMtx   sync.RWMutex // 保护state状态的读写锁
    	retryCount uint32       // 如果Permanent为true，retryCount记录该连接重复连接的次数
    }

还是先从ConnManager的Start()方法入手，分析其工作机制：

    func (cm *ConnManager) Start() {
    	// Already started?
    	if atomic.AddInt32(&cm.start, 1) != 1 {
    		return
    	}

    	log.Trace("Connection manager started")
    	cm.wg.Add(1)
    	go cm.connHandler()

    	// 只要调用者请求它们并启动所有侦听器，并在连接被接受时提供一个将被调用的回调函数。
    	if cm.cfg.OnAccept != nil {
    		for _, listner := range cm.cfg.Listeners {
    			cm.wg.Add(1)
    			go cm.listenHandler(listner)
    		}
    	}

    	for i := atomic.LoadUint64(&cm.connReqCount); i < uint64(cm.cfg.TargetOutbound); i++ {
    		go cm.NewConnReq()
    	}
    }

ConnMgr启动是主要有如下过程：

- 1.启动工作协程connHandler
    - go cm.connHandler()
- 2.启动监听协程listenHandler，等待其他节点连接
    - go cm.listenHandler(listner)
- 3.启动建立连接的协程，选择Peer地址并主动连接
    - go cm.NewConnReq()

onConnect,onDisconnect和OnAccept均在新的协程中回调，以免阻塞ConnMgr的工作协程和监听协程。

    func (cm *ConnManager) Connect(c *ConnReq) {
    	if atomic.LoadInt32(&cm.stop) != 0 {
    		return
    	}
    	if atomic.LoadUint64(&c.id) == 0 {
    		atomic.StoreUint64(&c.id, atomic.AddUint64(&cm.connReqCount, 1))

    		// 向连接管理器提交未决连接尝试的请求。通过在连接建立之前注册id，我们以后可以通过Remove方法取消连接。
    		done := make(chan struct{})
    		select {
    		case cm.requests <- registerPending{c, done}:
    		case <-cm.quit:
    			return
    		}

    		// 等待注册成功将待处理的连接请求添加到连接管理器的内部状态.
    		select {
    		case <-done:
    		case <-cm.quit:
    			return
    		}
    	}

    	log.Debugf("Attempting to connect to %v", c)

    	conn, err := cm.cfg.Dial(c.Addr)
    	if err != nil {
    		select {
    		case cm.requests <- handleFailed{c, err}:
    		case <-cm.quit:
    		}
    		return
    	}

    	select {
    	case cm.requests <- handleConnected{c, conn}:
    	case <-cm.quit:
    	}
    }

建立连接的过程就是调用指定的Dial()方法来进行TCP握手，如果Peer直连，则直接调用net.Dial()进行连接，
如果是通过代理与Peer连接，则会调用SOCKS Proxy的Dial()方法。根据是否连接成功向connHandler发送成功
或者失败的消息，让connHandler进一步处理。

- conn, err := cm.cfg.Dial(c.Addr)

    func (cm *ConnManager) Disconnect(id uint64) {
    	if atomic.LoadInt32(&cm.stop) != 0 {
    		return
    	}

    	select {
    	case cm.requests <- handleDisconnected{id, true}:
    	case <-cm.quit:
    	}
    }

调用Disconnect断开连接则向connHandler发送handleDisconnected消息让connHandler进一步处理。

- cm.requests <- handleDisconnected{id, true}:

所以从上可以看出，不管是连接还是断开，最后的处理逻辑都封装在connHandler中。

    func (cm *ConnManager) connHandler() {

    	var (
    		// 待处理的所有已注册的连接请求尚未成功。
    		pending = make(map[uint64]*ConnReq)

    		// conns表示所有主动连接的Peer的集合。
    		conns = make(map[uint64]*ConnReq, cm.cfg.TargetOutbound)
    	)

    out:
    	for {
    		select {
    		case req := <-cm.requests:
    			switch msg := req.(type) {

    			case registerPending:
    				connReq := msg.c
    				connReq.updateState(ConnPending)
    				pending[msg.c.id] = connReq
    				close(msg.done)

    			case handleConnected:
    				connReq := msg.c

    				if _, ok := pending[connReq.id]; !ok {
    					if msg.conn != nil {
    						msg.conn.Close()
    					}
    					log.Debugf("Ignoring connection for "+
    						"canceled connreq=%v", connReq)
    					continue
    				}

    				connReq.updateState(ConnEstablished)
    				connReq.conn = msg.conn
    				conns[connReq.id] = connReq
    				log.Debugf("Connected to %v", connReq)
    				connReq.retryCount = 0
    				cm.failedAttempts = 0

    				delete(pending, connReq.id)

    				if cm.cfg.OnConnection != nil {
    					go cm.cfg.OnConnection(connReq, msg.conn)
    				}

    			case handleDisconnected:
    				connReq, ok := conns[msg.id]
    				if !ok {
    					connReq, ok = pending[msg.id]
    					if !ok {
    						log.Errorf("Unknown connid=%d",
    							msg.id)
    						continue
    					}

    					// 发现挂起的连接，如果我们应该忽略稍后成功的连接，则将其从挂起的映射中移除。
    					connReq.updateState(ConnCanceled)
    					log.Debugf("Canceling: %v", connReq)
    					delete(pending, msg.id)
    					continue

    				}

    				// 找到现有连接，标记为断开连接并执行断开连接回调。
    				log.Debugf("Disconnected from %v", connReq)
    				delete(conns, msg.id)

    				if connReq.conn != nil {
    					connReq.conn.Close()
    				}

    				if cm.cfg.OnDisconnection != nil {
    					go cm.cfg.OnDisconnection(connReq)
    				}

    				// 所有内部状态已被清除，如果此连接被删除，我们将不会再尝试请求。
    				if !msg.retry {
    					connReq.updateState(ConnDisconnected)
    					continue
    				}

    				// 否则，如果我们没有足够的Peer，或者这是一个持久Peer，我们将尝试重新连接。
    				// 连接请求被重新添加到挂起的映射中，以便后续的连接和故障处理不会忽略该请求。
    				if uint32(len(conns)) < cm.cfg.TargetOutbound ||
    					connReq.Permanent {

    					connReq.updateState(ConnPending)
    					log.Debugf("Reconnecting to %v",
    						connReq)
    					pending[msg.id] = connReq
    					cm.handleFailedConn(connReq)
    				}

    			case handleFailed:
    				connReq := msg.c

    				if _, ok := pending[connReq.id]; !ok {
    					log.Debugf("Ignoring connection for "+
    						"canceled conn req: %v", connReq)
    					continue
    				}

    				connReq.updateState(ConnFailing)
    				log.Debugf("Failed to connect to %v: %v",
    					connReq, msg.err)
    				cm.handleFailedConn(connReq)
    			}

    		case <-cm.quit:
    			break out
    		}
    	}

    	cm.wg.Done()
    	log.Trace("Connection handler done")
    }

connHandler主要处理连接建立成功、失败和断开这三种情况：

- 1.连接成功
    - 更新连接的状态为ConnEstablished
    - 同时将该连接添加到conns中以跟踪它的后续状态
    - 将retryCount和failedAttempts重置
    - 在新的goroutine中回调OnConnection
- 2.断开连接
    - 先从conns找到要断开的connReq，更新状态为ConnDisconnected
    - 调用net.Conn的Close()方法断开TCP连接
    - 在新的goroutine中回调OnDisconnection
    - 在当前的活跃连接数少于设定的最大门限且retry为true时，则调用handleFailedConn进行重连或者选择新的Peer连接
- 3.连接失败
    - 将连接状态更新为ConnFailed，同时调用handleFailedConn进行重连或者选择新懂得Peer连接

在ConnMgr只处理连接建立成功或者失败的情况，如果连接一段时间后，出现中断的情况，TCP socket虽然有
keepalive选项开启心跳，但并没有心跳超时的回调，做法是当调用write()方法写入数据返回错误时检测到连接中断。

Peer之间会发送Ping/Pong心跳来维持及检测连接，如果Pong消息超时或者outHandler向net.Conn写数据出错时，Peer的
Disconnect()方法将会被调用以主动断开连接，并退出Peer的工作协程。

当Peer连接建立成功并回调OnConnect()时，server会新起一个goroutine守护与Peer的连接状态，当Peer
断开连接并退出时，server随即会调用ConnMgr的Disconnect()方法以清除该连接。

    func (cm *ConnManager) handleFailedConn(c *ConnReq) {
    	if atomic.LoadInt32(&cm.stop) != 0 {
    		return
    	}
    	if c.Permanent {
    		c.retryCount++
    		d := time.Duration(c.retryCount) * cm.cfg.RetryDuration
    		if d > maxRetryDuration {
    			d = maxRetryDuration
    		}
    		log.Debugf("Retrying connection to %v in %v", c, d)
    		time.AfterFunc(d, func() {
    			cm.Connect(c)
    		})
    	} else if cm.cfg.GetNewAddress != nil {
    		cm.failedAttempts++
    		if cm.failedAttempts >= maxFailedAttempts {
    			log.Debugf("Max failed connection attempts reached: [%d] "+
    				"-- retrying connection in: %v", maxFailedAttempts,
    				cm.cfg.RetryDuration)
    			time.AfterFunc(cm.cfg.RetryDuration, func() {
    				cm.NewConnReq()
    			})
    		} else {
    			go cm.NewConnReq()
    		}
    	}
    }

handleFailedConn负责处理重连逻辑：

- 如果连接的Permanent为true，则还是选择这个Peer进行连接，在次数和超时范围内进行重新连接
    - cm.Connect(c)
- 如果连接的Permannet为false，则动态选择新的Peer进行连接，如果尝试新连接的次数超限（默认是25次），推断节点的出口网络有问题，延时连接，默认延时5s
    - cm.NewConnReq()

    func (cm *ConnManager) NewConnReq() {
    	if atomic.LoadInt32(&cm.stop) != 0 {
    		return
    	}
    	if cm.cfg.GetNewAddress == nil {
    		return
    	}

    	c := &ConnReq{}
    	atomic.StoreUint64(&c.id, atomic.AddUint64(&cm.connReqCount, 1))

    	// 向连接管理器提交未决连接尝试的请求。通过在连接建立之前注册id，我们以后可以通过Remove方法取消连接。
    	done := make(chan struct{})
    	select {
    	case cm.requests <- registerPending{c, done}:
    	case <-cm.quit:
    		return
    	}

    	// 等待注册成功将待处理的连接请求添加到连接管理器的内部状态。
    	select {
    	case <-done:
    	case <-cm.quit:
    		return
    	}

    	addr, err := cm.cfg.GetNewAddress()
    	if err != nil {
    		select {
    		case cm.requests <- handleFailed{c, err}:
    		case <-cm.quit:
    		}
    		return
    	}
    	c.Addr = addr
    	cm.Connect(c)
    }

动态选择新的Peer进行连接：

- 新建ConnReq对象，并为其分配一个id
    - c := &ConnReq{}
    - atomic.StoreUint64(&c.id, atomic.AddUint64(&cm.connReqCount, 1))
- 通过GetNewAddress()从addrmgr维护的地址仓库中随机选择一个Peer的可达地址，如果地址选择失败，则由connHandler再次发起新的连接
    - addr, err := cm.cfg.GetNewAddress()
    - case cm.requests <- handleFailed{c, err}:
- 调用Connect()方法开始与Peer建立连接
    - cm.Connect(c)

已经大概了解了ConnMgr主动与Peer建立连接，及失败后重连或者选择新地址连接的过程，
接下来通过listenHandler来看它被动等待连接的实现：

    func (cm *ConnManager) listenHandler(listener net.Listener) {
    	log.Infof("Server listening on %s", listener.Addr())
    	for atomic.LoadInt32(&cm.stop) == 0 {
    		conn, err := listener.Accept()
    		if err != nil {
    			// Only log the error if not forcibly shutting down.
    			if atomic.LoadInt32(&cm.stop) == 0 {
    				log.Errorf("Can't accept connection: %v", err)
    			}
    			continue
    		}
    		go cm.cfg.OnAccept(conn)
    	}

    	cm.wg.Done()
    	log.Tracef("Listener handler done for %s", listener.Addr())
    }

listenHandler主要是等待连接，连接成功后在新协程中回调OnAccept。
    - go cm.cfg.OnAccept(conn)

后续将继续介绍OnConnect和OnAccept回调。

上面介绍的还只是ConnMgr建立及维护连接的主要过程，接下来，通过dynamicbanscore.go来分析
用于防止DDoS攻击的动态记分器是如何实现的，先看DynamicBanScore的定义：

    type DynamicBanScore struct {
    	lastUnix   int64        // 上一次调整分值的Unix时间点
    	transient  float64      // 分值的浮动衰减的部分
    	persistent uint32       // 分值中不会自动衰减的部分
    	mtx        sync.Mutex   // 保护transient和persistent的互斥锁
    }

DynamicBanScore提供的分值是由一个不变值和瞬时值构成的，那么这两值到底拼装出结果：

    func (s *DynamicBanScore) int(t time.Time) uint32 {
    	dt := t.Unix() - s.lastUnix
    	if s.transient < 1 || dt < 0 || Lifetime < dt {
    		return s.persistent
    	}
    	return s.persistent + uint32(s.transient*decayFactor(dt))
    }

最后的分值等于persistent加上transient乘以一个衰减系数后的和，衰减系数随时间变化，定义了一个方法：

    func decayFactor(t int64) float64 {
    	if t < precomputedLen {
    		return precomputedFactor[t]
    	}
    	return math.Exp(-1.0 * float64(t) * lambda)
    }

衰减系数是按时间间隔呈现指数分布的。

- dt := t.Unix() - s.lastUnix
- lambda = math.Ln2 / Halflife
- score = persistent + transistent * e(-lambda*Δt)

这里的时间间隔是指当前取值时刻距上一次主动调用persistent或者transient值的时间差。

    func (s *DynamicBanScore) increase(persistent, transient uint32, t time.Time) uint32 {
    	s.persistent += persistent
    	tu := t.Unix()
    	dt := tu - s.lastUnix

    	if transient > 0 {
    		if Lifetime < dt {
    			s.transient = 0
    		} else if s.transient > 1 && dt > 0 {
    			s.transient *= decayFactor(dt)
    		}
    		s.transient += float64(transient)
    		s.lastUnix = tu
    	}
    	return s.persistent + uint32(s.transient)
    }

- 主动调节score值时，现将persistent值相加
    - s.persistent += persistent
- 计算出传入时刻t的transient值
    - s.transient *= decayFactor(dt)
- 在与传入的transient值相加后得到新的transient值
    - s.transient += float64(transient)
- 新persistent和新transient相加得到score
    - return s.persistent + uint32(s.transient)

Peer之间交换消息时，每一个Peer连接会有一个动态记分器来监控它们之间收发消息的频率，太频繁的收到某个Peer发送过来的消息，
将会被怀疑遭到DDoS攻击，从而主动断开与它的连接。

ConnMgr会通过GetNewAddrss()来选取Peer的地址，但一个新的节点接入时，还没有与任何Peer交换过地址信息，所以其地址仓库是空的，
那它该与哪些节点建立连接，实际上，节点会内置一些种子节点的地址。

    var MainNetParams = Params{
    	Name:        "mainnet",
    	Net:         wire.MainNet,
    	DefaultPort: "8333",
    	DNSSeeds: []DNSSeed{
    		{"seed.bitcoin.sipa.be", true},
    		{"dnsseed.bluematt.me", true},
    		{"dnsseed.bitcoin.dashjr.org", false},
    		{"seed.bitcoinstats.com", true},
    		{"seed.bitnodes.io", false},
    		{"seed.bitcoin.jonasschnelli.ch", true},
    	},
    ...

在 btcd/chaincfg/params.go这个脚本里，直接内置了6个种子节点的域名。

在ConnMgr连接种子节点之前，必须进行DNS Lookup查询对应的IP地址，这是在SeedFromDNS()中完成的。

    if !cfg.DisableDNSSeed {
    		// Add peers discovered through DNS to the address manager.
    		connmgr.SeedFromDNS(activeNetParams.Params, defaultRequiredServices,
    			btcdLookup, func(addrs []*wire.NetAddress) {
    				// Bitcoind在这里使用dns seeder的查找。这很奇怪，因为DNS种子查找的值会有很大的变化，为了复制这种行为，我们把所有的地址都放在第一个地址上。
    				s.addrManager.AddAddresses(addrs, addrs[0])
    			})
    	}
    	go s.connManager.Start()

在server.go 的peerHandler()方法中，connmgr有一个SeedFromDNS方法。

    func SeedFromDNS(chainParams *chaincfg.Params, reqServices wire.ServiceFlag,
    	lookupFn LookupFunc, seedFn OnSeed) {
    	for _, dnsseed := range chainParams.DNSSeeds {
    		var host string
    		if !dnsseed.HasFiltering || reqServices == wire.SFNodeNetwork {
    			host = dnsseed.Host
    		} else {
    			host = fmt.Sprintf("x%x.%s", uint64(reqServices), dnsseed.Host)
    		}

    		go func(host string) {
    			randSource := mrand.New(mrand.NewSource(time.Now().UnixNano()))
    			seedpeers, err := lookupFn(host)
    			if err != nil {
    				log.Infof("DNS discovery failed on seed %s: %v", host, err)
    				return
    			}
    			numPeers := len(seedpeers)

    			log.Infof("%d addresses found from DNS seed %s", numPeers, host)

    			if numPeers == 0 {
    				return
    			}
    			addresses := make([]*wire.NetAddress, len(seedpeers))
    			// if this errors then we have *real* problems
    			intPort, _ := strconv.Atoi(chainParams.DefaultPort)
    			for i, peer := range seedpeers {
    				addresses[i] = wire.NewNetAddressTimestamp(
    					// bitcoind seeds with addresses from a time randomly selected between 3 and 7 days ago.
    					time.Now().Add(-1*time.Second*time.Duration(secondsIn3Days+
    						randSource.Int31n(secondsIn4Days))),
    					0, peer, uint16(intPort))
    			}
    			seedFn(addresses)
    		}(host)
    	}
    }

其主要步骤为：

- 1.调用lookupFn()进行DNS resolve，将种子节点的域名解析了IP地址
- 2.将种子节点的IP地址封装为协议地址wire.NetAddress，其中主要是增加了地址的时效性，这里将地址的时效性随机地设为3-7天。

这里传入的方法lookupFn()根据配置，有可能是节点自己访问DNS Server解析，也有可能通过洋葱代理进行解析。

在/btcd/config.go脚本中：

    func loadConfig() (*config, []string, error) {

        ......

        // 设置拨号和DNS解析（查找）功能取决于指定的选项。默认是使用标准net.DialTimeout函数以及系统DNS解析器。当指定代理时，
        // 拨号功能被设置为代理特定的拨号功能并且查找被设置为使用tor（除非--noonion被指定，在这种情况下使用系统DNS解析器）。
        cfg.dial = net.DialTimeout
        cfg.lookup = net.LookupIP
        if cfg.Proxy != "" {
            _, _, err := net.SplitHostPort(cfg.Proxy)

            ......

            // Tor隔离标志意味着代理证书将被覆盖，除非还配置了一个洋葱代理，在这种情况下将覆盖一个代理.
            torIsolation := false
            if cfg.TorIsolation && cfg.OnionProxy == "" &&
                (cfg.ProxyUser != "" || cfg.ProxyPass != "") {

                torIsolation = true
                fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
                    "overriding specified proxy user credentials")
            }

            proxy := &socks.Proxy{
                Addr:         cfg.Proxy,
                Username:     cfg.ProxyUser,
                Password:     cfg.ProxyPass,
                TorIsolation: torIsolation,
            }
            cfg.dial = proxy.DialTimeout

            // 将代理视为tor并通过它执行DNS解析，除非设置了--noonion标志或配置了特定于洋葱的代理。
            if !cfg.NoOnion && cfg.OnionProxy == "" {
                cfg.lookup = func(host string) ([]net.IP, error) {
                    return connmgr.TorLookupIP(host, cfg.Proxy)
                }
            }
        }

        // 根据指定的选项设置洋葱地址拨号功能。默认是使用上面选择的相同拨号功能。但是，当指定洋葱特定代理时，
        // 洋葱地址拨号功能被设置为使用洋葱专用代理，同时保留上面选择的正常拨号功能。
        // 这允许.onion地址流量通过与正常流量不同的代理进行路由。
        if cfg.OnionProxy != "" {
            _, _, err := net.SplitHostPort(cfg.OnionProxy)

            ......

            cfg.oniondial = func(network, addr string, timeout time.Duration) (net.Conn, error) {
                proxy := &socks.Proxy{
                    Addr:         cfg.OnionProxy,
                    Username:     cfg.OnionProxyUser,
                    Password:     cfg.OnionProxyPass,
                    TorIsolation: cfg.TorIsolation,
                }
                return proxy.DialTimeout(network, addr, timeout)
            }

            // 当配置为网桥模式（配置了--onion和--proxy）时，这意味着由--proxy配置的代理不是tor代理，因此覆盖DNS解析以使用特定于洋葱的代理。
            if cfg.Proxy != "" {
                cfg.lookup = func(host string) ([]net.IP, error) {
                    return connmgr.TorLookupIP(host, cfg.OnionProxy)
                }
            }
        } else {
            cfg.oniondial = cfg.dial
        }

        // 指定--noonion表示洋葱地址拨号功能会导致错误。
        if cfg.NoOnion {
            cfg.oniondial = func(a, b string, t time.Duration) (net.Conn, error) {
                return nil, errors.New("tor has been disabled")
            }
        }

        ......
    }

上面代码看出：

- 1.默认的DNS Lookup和Dial方法就是标准的net.LookupIP和net.DialTimeout
- 2.如果设置了代理，Dial方法将使用SOCKS Proxy的DialTimeout()，如果未禁用洋葱代理，则默认代理为洋葱代理，DNS查询将通过connmgr的TorLookupIP()实现
- 3.如果专门设置了洋葱代理，则设定对“暗网”服务（hidden service）的连接采用SOCKS Proxy的DialTimeout()，DNS Lookup将使用connmgr的TorLookupIP()

无论是通过普通代理还是洋葱代理连接Peer，对节点来说，她们均是SOCKS代理服务器，节点与他们之间通过SOCKS协议来通信。
与普通代理相比，洋葱代理扩展了SOCKS协议，加入了对[Name lookup、Stream Isolation](https://gitweb.torproject.org/torspec.git/tree/socks-extensions.txt)等的支持。
SOCKS协议位于会话层，在传输层与应用层之间，所以它不仅可以代理HTTP流量，也可以代理如FTP，XMPP等其他应用流量。

为了了解btcd如何通过SOCKS代理建立连接，看看Proxy的dial()方法：


    func (p *Proxy) dial(network, addr string, timeout time.Duration) (net.Conn, error) {
        host, strPort, err := net.SplitHostPort(addr)
        if err != nil {
            return nil, err
        }
        port, err := strconv.Atoi(strPort)
        if err != nil {
            return nil, err
        }

        conn, err := net.DialTimeout("tcp", p.Addr, timeout)
        if err != nil {
            return nil, err
        }

        var user, pass string
        if p.TorIsolation {                                                  (2)
            var b [16]byte
            _, err := io.ReadFull(rand.Reader, b[:])
            if err != nil {
                conn.Close()
                return nil, err
            }
            user = hex.EncodeToString(b[0:8])
            pass = hex.EncodeToString(b[8:16])
        } else {
            user = p.Username
            pass = p.Password
        }
        buf := make([]byte, 32+len(host)+len(user)+len(pass))

        // Initial greeting
        buf[0] = protocolVersion
        if user != "" {
            buf = buf[:4]
            buf[1] = 2 // num auth methods
            buf[2] = authNone
            buf[3] = authUsernamePassword
        } else {
            buf = buf[:3]
            buf[1] = 1 // num auth methods
            buf[2] = authNone
        }

        _, err = conn.Write(buf)
        if err != nil {
            conn.Close()
            return nil, err
        }

        // Server's auth choice

        if _, err := io.ReadFull(conn, buf[:2]); err != nil {
            conn.Close()
            return nil, err
        }
        if buf[0] != protocolVersion {
            conn.Close()
            return nil, ErrInvalidProxyResponse
        }
        err = nil
        switch buf[1] {
        default:
            err = ErrInvalidProxyResponse
        case authUnavailable:
            err = ErrNoAcceptableAuthMethod
        case authGssApi:
            err = ErrNoAcceptableAuthMethod
        case authUsernamePassword:
            buf = buf[:3+len(user)+len(pass)]
            buf[0] = 1 // version
            buf[1] = byte(len(user))
            copy(buf[2:], user)
            buf[2+len(user)] = byte(len(pass))
            copy(buf[3+len(user):], pass)
            if _, err = conn.Write(buf); err != nil {
                conn.Close()
                return nil, err
            }
            if _, err = io.ReadFull(conn, buf[:2]); err != nil {
                conn.Close()
                return nil, err
            }
            if buf[0] != 1 { // version
                err = ErrInvalidProxyResponse
            } else if buf[1] != 0 { // 0 = succes, else auth failed
                err = ErrAuthFailed
            }
        case authNone:
            // Do nothing
        }
        if err != nil {
            conn.Close()
            return nil, err
        }

        // Command / connection request

        buf = buf[:7+len(host)]
        buf[0] = protocolVersion
        buf[1] = commandTcpConnect
        buf[2] = 0 // reserved
        buf[3] = addressTypeDomain
        buf[4] = byte(len(host))
        copy(buf[5:], host)
        buf[5+len(host)] = byte(port >> 8)
        buf[6+len(host)] = byte(port & 0xff)
        if _, err := conn.Write(buf); err != nil {
            conn.Close()
            return nil, err
        }

        // Server response

        if _, err := io.ReadFull(conn, buf[:4]); err != nil {
            conn.Close()
            return nil, err
        }

        if buf[0] != protocolVersion {
            conn.Close()
            return nil, ErrInvalidProxyResponse
        }

        if buf[1] != statusRequestGranted {
            conn.Close()
            err := statusErrors[buf[1]]
            if err == nil {
                err = ErrInvalidProxyResponse
            }
            return nil, err
        }

        paddr := &ProxiedAddr{Net: network}

        switch buf[3] {                                                      (6)
        default:
            conn.Close()
            return nil, ErrInvalidProxyResponse
        case addressTypeIPv4:
            if _, err := io.ReadFull(conn, buf[:4]); err != nil {
                conn.Close()
                return nil, err
            }
            paddr.Host = net.IP(buf).String()
        case addressTypeIPv6:
            if _, err := io.ReadFull(conn, buf[:16]); err != nil {
                conn.Close()
                return nil, err
            }
            paddr.Host = net.IP(buf).String()
        case addressTypeDomain:
            if _, err := io.ReadFull(conn, buf[:1]); err != nil {
                conn.Close()
                return nil, err
            }
            domainLen := buf[0]
            if _, err := io.ReadFull(conn, buf[:domainLen]); err != nil {
                conn.Close()
                return nil, err
            }
            paddr.Host = string(buf[:domainLen])
        }

        if _, err := io.ReadFull(conn, buf[:2]); err != nil {
            conn.Close()
            return nil, err
        }
        paddr.Port = int(buf[0])<<8 | int(buf[1])

        return &proxiedConn{                                                 (7)
            conn:       conn,
            boundAddr:  paddr,
            remoteAddr: &ProxiedAddr{network, host, port},
        }, nil
    }

在btcsuite/go-socks/dial.go脚本，可以看到由于btcd节点之间均通过TCP连接，因此这里实现的是SOCKS代理TCP连接的情形。建立代理连接的主要步骤为：

- 1.与SOCKS代理服务器建立TCP连接
    -  conn, err := net.DialTimeout("tcp", p.Addr, timeout)
- 2.客户端向代理服务器发送协议版本和METHOD集合的协商请求
    - buf[0] = protocolVersion
- 3.等待SOCKS服务器响应，如果SOCKS服务器不支持SOCKS 5，则协商失败；如果支持并同意不验证，则客户端可以直接发送后续请求，如要要用户名/密码验证，客户端随后提交验证。
    - buf = buf[:3+len(user)+len(pass)]
- 4.无需要认证或者用户名/密码验证通过后，客户端向SOCKS客户端发送CONNECT请求，并指明目的IP和端口号。
    - buf = buf[:7+len(host)]
- 5.SOCKS服务器响应CONNECT请求，如果代理连接成功，则返回外部的代理地址和端口。
- 6.创建并返回一个代理连接对象proxiedConn，它的conn字段描述客户端与SOCKS服务器的TCP连接，该连接上的TCP报文将通过代理服务器转发给目的地址，boundAddr描述代理的外部地址和端口，remoteAddr描述目的地址和端口。

如果客户端连接一个Tor代理，并且希望开启Stream Isolation特性，则随机生成用户名和密码并发往Tor代理服务器。
Stream Isolation是为了禁止Tor网络在同一个“虚电路”上中继不同的TCP流，
Tor代理服务器支持通过IsolateClientAddr、IsolateSOCKSAuth、IsolateClientProtocol、IsolateDestPort及IsolateDestAddr等方式来标识不同的TCP流。
Btcd选择通过IsolateSOCKSAuth来支持Stream Isolation，使得同一节点在连接不同Peer或者重连相同Peer时的TCP在Tor网络中均能被“隔离”。
然而，读者可能会产生疑问: 随机生成的用户名和密码如何被Tor代理服务器验证？
实际上，Btcd这里使用随机用户名和密码，是要求Tor代理服务器作如下配置: 选择“NO AUTHENTICATION REQUIRED”作为验证方式，并且只通过username来标识不同代理请求。

了解了通过SOCKS代理或者Tor代理与Peer建立TCP连接的机制后，就可以来看看如果通过Tor代理来进行DNS查询。

通过Tor代理进行DNS查询不是解析洋葱地址，而是解析“明网”中的域名。

例如用户通过Tor代理访问www.twitter.com时，用户可以选择先通过DNS查询到IP地址后，再通过Tor代理连接到该IP地址。

也可以将该域名作为目的地址发给Tor代理，让Tor网络的退出节点进行DNS查询，并建立与目的地址的连接。

如果客户端不想向DNS Server暴露自己的目标访问域名，同时希望进行域名解析，那么它可以通过Tor代理进行DNS解析。

btcd/connmgr/tor.go

    func TorLookupIP(host, proxy string) ([]net.IP, error) {
        conn, err := net.Dial("tcp", proxy)
        if err != nil {
            return nil, err
        }
        defer conn.Close()

        buf := []byte{'\x05', '\x01', '\x00'}
        _, err = conn.Write(buf)
        if err != nil {
            return nil, err
        }

        buf = make([]byte, 2)
        _, err = conn.Read(buf)
        if err != nil {
            return nil, err
        }
        if buf[0] != '\x05' {
            return nil, ErrTorInvalidProxyResponse
        }
        if buf[1] != '\x00' {
            return nil, ErrTorUnrecognizedAuthMethod
        }

        buf = make([]byte, 7+len(host))
        buf[0] = 5      // protocol version
        buf[1] = '\xF0' // Tor Resolve
        buf[2] = 0      // reserved
        buf[3] = 3      // Tor Resolve
        buf[4] = byte(len(host))
        copy(buf[5:], host)
        buf[5+len(host)] = 0 // Port 0

        _, err = conn.Write(buf)
        if err != nil {
            return nil, err
        }

        buf = make([]byte, 4)
        _, err = conn.Read(buf)
        if err != nil {
            return nil, err
        }
        if buf[0] != 5 {
            return nil, ErrTorInvalidProxyResponse
        }
        if buf[1] != 0 {
            if int(buf[1]) >= len(torStatusErrors) {
                return nil, ErrTorInvalidProxyResponse
            } else if err := torStatusErrors[buf[1]]; err != nil {
                return nil, err
            }
            return nil, ErrTorInvalidProxyResponse
        }
        if buf[3] != 1 {
            err := torStatusErrors[torGeneralError]
            return nil, err
        }

        buf = make([]byte, 4)
        bytes, err := conn.Read(buf)
        if err != nil {
            return nil, err
        }
        if bytes != 4 {
            return nil, ErrTorInvalidAddressResponse
        }

        r := binary.BigEndian.Uint32(buf)

        addr := make([]net.IP, 1)
        addr[0] = net.IPv4(byte(r>>24), byte(r>>16), byte(r>>8), byte(r))

        return addr, nil
    }

其过程与建立代理连接的方式类似，即先协商版本与认证方式，再发送请求与等待响应。不同的地方在于：

- 1.选择不认证的方式
    -  buf := []byte{'\x05', '\x01', '\x00'}
- 2.请求的命令是'FO'，它是Tor代理扩展的命令，指明用于Name Lookup，同时目标地址类型指定为DOMAINNAME
    -  buf[1] = '\xF0'
- 3.Tor退出节点进行DNS查询后，由Tor代码返回。这里仅接受IPv4地址
    - if buf[3] != 1

我们完整分析了Bitcoin P2P网络中Peer节点之间建立，维持和断开TCP连接的所有过程，包括了通过SOCKS代理或Tor代理进行连接或DNS查询的实现。

除了节点内置的种子节点的地址，节点接入网络时并不知道其他节点的地址，那么节点是如何知道网络中其他节点的地址，以及如何选择Peer节点地址
建立连接呢？

Peer节会维护一个记录网络节点地址的地址仓库。节点与Peer交换getaddr和addr消息同步各自已知的节点地址，一段时间后，节点将获得大量
的节点地址。这些已知的节点地址呢，就需要一个“仓库”来记录这些地址，并且在节点需要与新的节点建立Peer关系时能够随机选择可用的
地址以供连接。AddrManager完成了这些功能。

### btcd/addrmgr

此目录下包含的文件有：

- addrmanager.go:实现Peer地址的存取以及随机选择策略，是AddrManager的主要模块，将地址集合以特定的形式存于peers.json文件中。
- knownaddress.go:定义了knownAddress类型，即地址仓库中每条地址记录的格式。
- network.go:定义了不同IP地址类型，并提供类型判断方法。
- log.go:提供logger初始化和设置方法。
- cov_report.sh:调用gocov生成测试覆盖报告的脚本。

AddrManger主要将节点通过addr消息获知的地址存入本地的peers.json文件，首先看看peers.json的格式：

    {
        "Version": 1,
        "Key": [233,19,87,131,183,155,......,231,78,82,150,10,102],
        "Addresses": [
            {
                "Addr": "109.157.120.169:8333",
                "Src": "104.172.5.90:8333",
                "Attempts": 0,
                "TimeStamp": 1514967959,
                "LastAttempt": -62135596800,
                "LastSuccess": -62135596800
            },
            ......
        ],
        "NewBuckets": [
            [
                "[2001:0:9d38:78cf:3cb1:bb2:ab6f:e8b4]:8333",
                "196.209.239.229:8333",
                ......
                "65.130.177.198:8333"
            ],
            ......
            [
                "125.227.159.115:8333",
                ......
                "alhlegtjkdmbqsvt.onion:8333",
                ......
                "79.250.188.226:8333"
            ]
        ],
        "TriedBuckets": [
            [
                "5.9.165.181:8333",
                ......
                "5.9.17.24:8333"
            ],
            [
                "95.79.50.90:8333",
                ......
                "[2a02:c207:2008:9136::1]:8333"
            ]
        ]
    }

地址仓库（peers.json）中包含version，随机序列key及Addresses，NewBuckets和TriedBuckets等，这些可以对应到serializedAddrManager的定义：

    type serializedAddrManager struct {
    	Version      int
    	Key          [32]byte
    	Addresses    []*serializedKnownAddress
    	NewBuckets   [newBucketCount][]string // string is NetAddressKey
    	TriedBuckets [triedBucketCount][]string
    }

在/btcd/addrmgr/addrmanager.go中，serializedAddrManager的定义如上，serializedKnownAddress的定义如下：

    type serializedKnownAddress struct {
    	Addr        string
    	Src         string
    	Attempts    int
    	TimeStamp   int64
    	LastAttempt int64
    	LastSuccess int64
    	// no refcount or tried, that is available from context.
    }

上面两个结构分别对应peers.json中的Addresses字段记录的地址集和KnownAddress。

    type KnownAddress struct {
    	na          *wire.NetAddress
    	srcAddr     *wire.NetAddress
    	attempts    int
    	lastattempt time.Time
    	lastsuccess time.Time
    	tried       bool
    	refs        int // reference count of new buckets
    }

在/btcd/addrmgr/knownaddress.go中，其各个字段属性意义如下：

- na：从addr消息获知的节点的IPv4或者IPv6地址，经过序列化后，在peers.json中有".onion"的地址，由支持Tor的IPv6地址转换而来。
- srcAddr：addr消息的源，也就是当前节点的Peer。
- attempts：连接成功之前尝试连接的次数。
- lastattempt：最近一次尝试连接的时间点。
- lastsuccess：最近一次尝试连接成功的时间点。
- tried：标识是否已经尝试连接过，已经tried过的地址将被放入TriedBuckets。
- refs：该地址所属的NewBucket的个数，默认最大个数是8。同一个地址从不同的scrAddr的Peer获得，所以可能同一个地址也放入不同的NewBucket中。

从上面可以看到AddrManager的地址仓库的形式和它管理的地址类型的定义，关键是AddrManager如何来存取这些地址。

    type AddrManager struct {
    	mtx            sync.Mutex                               //AddrManager的对象锁，保证addrManager是并发安全的
    	peersFile      string                                   //存储地址仓库的文件名，默认是peers.json
    	lookupFunc     func(string) ([]net.IP, error)           //进行DNS Lookup的函数值
    	rand           *rand.Rand                               //随机数生成器
    	key            [32]byte                                 //32字节的随机数序列，用于计算NewBucket和TriedBucket的索引
    	addrIndex      map[string]*KnownAddress                 //缓存所有KnownAddress的map
    	addrNew        [newBucketCount]map[string]*KnownAddress //缓存所有新地址的map slice
    	addrTried      [triedBucketCount]*list.List             //缓存所有已经Tried的地址的list slice。上面的map和这里的list都可以遍历
    	started        int32                                    //用于标识addrmanager已经启动
    	shutdown       int32                                    //用于标识addrmanager已经停止
    	wg             sync.WaitGroup                           //用于同步退出，addrmanager停止时等待工作协程退出
    	quit           chan struct{}                            //用于通知工作协程退出
    	nTried         int                                      //记录Tried地址个数
    	nNew           int                                      //记录New地址个数
    	lamtx          sync.Mutex                               //保护localAddresses的互斥锁
    	localAddresses map[string]*localAddress                 //保存已知的本地地址
    }

先来分析一下AddrManager的Start(),AddAddress()和GetAddress(),Good()等方法来了解地址管理是如何进行工作的。

    func (a *AddrManager) Start() {
    	// Already started?
    	if atomic.AddInt32(&a.started, 1) != 1 {
    		return
    	}
    	log.Trace("Starting address manager")
    	// Load peers we already know about from file.
    	a.loadPeers()
    	// Start the address ticker to save addresses periodically.
    	a.wg.Add(1)
    	go a.addressHandler()
    }

Start()方法很简单，调用loadPeers()来将peers.json文件中的地址集实例化，然后启动工作协程addressHandler来周期性
向文件保存新的地址。loadPeers()主要是调用deserializePeers()将文件反序列化：

    func (a *AddrManager) deserializePeers(filePath string) error {
    	_, err := os.Stat(filePath)
    	if os.IsNotExist(err) {
    		return nil
    	}
    	r, err := os.Open(filePath)
    	if err != nil {
    		return fmt.Errorf("%s error opening file: %v", filePath, err)
    	}
    	defer r.Close()

    	var sam serializedAddrManager
    	dec := json.NewDecoder(r)
    	err = dec.Decode(&sam)
    	if err != nil {
    		return fmt.Errorf("error reading %s: %v", filePath, err)
    	}

    	if sam.Version != serialisationVersion {
    		return fmt.Errorf("unknown version %v in serialized "+
    			"addrmanager", sam.Version)
    	}
    	copy(a.key[:], sam.Key[:])

    	for _, v := range sam.Addresses {
    		ka := new(KnownAddress)
    		ka.na, err = a.DeserializeNetAddress(v.Addr)
    		if err != nil {
    			return fmt.Errorf("failed to deserialize netaddress "+
    				"%s: %v", v.Addr, err)
    		}
    		ka.srcAddr, err = a.DeserializeNetAddress(v.Src)
    		if err != nil {
    			return fmt.Errorf("failed to deserialize netaddress "+
    				"%s: %v", v.Src, err)
    		}
    		ka.attempts = v.Attempts
    		ka.lastattempt = time.Unix(v.LastAttempt, 0)
    		ka.lastsuccess = time.Unix(v.LastSuccess, 0)
    		a.addrIndex[NetAddressKey(ka.na)] = ka
    	}

    	for i := range sam.NewBuckets {
    		for _, val := range sam.NewBuckets[i] {
    			ka, ok := a.addrIndex[val]
    			if !ok {
    				return fmt.Errorf("newbucket contains %s but "+
    					"none in address list", val)
    			}

    			if ka.refs == 0 {
    				a.nNew++
    			}
    			ka.refs++
    			a.addrNew[i][val] = ka
    		}
    	}
    	for i := range sam.TriedBuckets {
    		for _, val := range sam.TriedBuckets[i] {
    			ka, ok := a.addrIndex[val]
    			if !ok {
    				return fmt.Errorf("Newbucket contains %s but "+
    					"none in address list", val)
    			}

    			ka.tried = true
    			a.nTried++
    			a.addrTried[i].PushBack(ka)
    		}
    	}

    	// Sanity checking.
    	for k, v := range a.addrIndex {
    		if v.refs == 0 && !v.tried {
    			return fmt.Errorf("address %s after serialisation "+
    				"with no references", k)
    		}

    		if v.refs > 0 && v.tried {
    			return fmt.Errorf("address %s after serialisation "+
    				"which is both new and tried!", k)
    		}
    	}

    	return nil
    }

从peers.json文件中读取数据，然后反序列化，其主要过程如下：

- 1.读取文件，通过json解析器将json文件实例化为serializedAddrManager对象
    - dec := json.NewDecoder(r)
    - err = dec.Decode(&sam)
- 2.校验版本号，并读取随机数序列Key
    - sam.Version != serialisationVersion
- 3.将serializedKnownAddress解析为KnownAddress，并存入AddrManager.addrIndexx中。DeserializeNetAddress方法将字符串地址转成所需的*wire.NetAddress
    - ka := new(KnownAddress)
    - ka.na, err = a.DeserializeNetAddress(v.Addr)
    - a.addrIndex[NetAddressKey(ka.na)] = ka
- 4.以serializedAddrManager的NewBuckets和TriedBuckets中的地址为Key，查找addrIndex中对应的KnownAddress后，填充addrNew和addrTried
    - a.addrNew[i][val] = ka        // map的slice
    - a.addrTried[i].PushBack(ka)   // list的slice
- 5.最后对实例化的结果作Sanity检查，保证一个地址要么在NewBuckets中，要么在TriedBuckets中
    - for k, v := range a.addrIndex

AddrManager启动后通过loadPeers()将文件中的记录实例化后，接着就启动了一个工作协程addressHandler：

    func (a *AddrManager) addressHandler() {
    	dumpAddressTicker := time.NewTicker(dumpAddressInterval)
    	defer dumpAddressTicker.Stop()
    out:
    	for {
    		select {
    		case <-dumpAddressTicker.C:
    			a.savePeers()

    		case <-a.quit:
    			break out
    		}
    	}
    	a.savePeers()
    	a.wg.Done()
    	log.Trace("Address handler done")
    }

这个方法是启动一个协程，每隔dumpAddressInterval（默认是10分钟）调用savePeers()将addrManager中的地址集合写入文件。

savePeers()方法就需要将对象进行序列化写入到文件，其步骤如下：

- 1.a.mtx.Lock()  先上锁
- 2.sam := new(serializedAddrManager)   声明一个可序列化的数据结果装AddrManager
- 3.sam.Addresses = make([]*serializedKnownAddress, len(a.addrIndex))   声明一个slice来装Addresses
- 4.for k, v := range a.addrIndex { ska := new(serializedKnownAddress)  对addrmanager进行遍历，声明可序列化的KnownAddress，然后填充，装到Addresses中，sam.Addresses[i] = ska
- 5.for i := range a.addrNew { sam.NewBuckets[i] = make([]string, len(a.addrNew[i])) 遍历addrmanager，对NewBuckets进行声明，填充sam.NewBuckets[i][j] = k
- 6.for i := range a.addrTried {sam.TriedBuckets[i] = make([]string, a.addrTried[i].Len()) 遍历addrmanager，对TriedBuckets进行声明，填充sam.TriedBuckets[i][j] = NetAddressKey(ka.na)
- 7.w, err := os.Create(a.peersFile) 打开文件
- 8.enc := json.NewEncoder(w)   进行encoder编码
- 9.if err := enc.Encode(&sam); err != nil {    写入文件进行保存

节点与Peer之间交换getaddr和addr消息时，会收到来自Peer告知的地址信息，这些地址会通过addrManager的AddAddress()或者
AddAddresses()方法添加到addrManager的地址集合中。实际上，AddAddress()或者AddAddresses()会通过调用updateAddress()来做更新操作。

    func (a *AddrManager) updateAddress(netAddr, srcAddr *wire.NetAddress) {
    	// Filter out non-routable addresses. Note that non-routable
    	// also includes invalid and local addresses.
    	if !IsRoutable(netAddr) {
    		return
    	}

    	addr := NetAddressKey(netAddr)
    	ka := a.find(netAddr)
    	if ka != nil {
    		// TODO: only update addresses periodically.
    		// Update the last seen time and services.
    		// note that to prevent causing excess garbage on getaddr
    		// messages the netaddresses in addrmaanger are *immutable*,
    		// if we need to change them then we replace the pointer with a
    		// new copy so that we don't have to copy every na for getaddr.
    		if netAddr.Timestamp.After(ka.na.Timestamp) ||
    			(ka.na.Services&netAddr.Services) !=
    				netAddr.Services {

    			naCopy := *ka.na
    			naCopy.Timestamp = netAddr.Timestamp
    			naCopy.AddService(netAddr.Services)
    			ka.na = &naCopy
    		}

    		// If already in tried, we have nothing to do here.
    		if ka.tried {
    			return
    		}

    		// Already at our max?
    		if ka.refs == newBucketsPerAddress {
    			return
    		}

    		// The more entries we have, the less likely we are to add more.
    		// likelihood is 2N.
    		factor := int32(2 * ka.refs)
    		if a.rand.Int31n(factor) != 0 {
    			return
    		}
    	} else {
    		// Make a copy of the net address to avoid races since it is
    		// updated elsewhere in the addrmanager code and would otherwise
    		// change the actual netaddress on the peer.
    		netAddrCopy := *netAddr
    		ka = &KnownAddress{na: &netAddrCopy, srcAddr: srcAddr}
    		a.addrIndex[addr] = ka
    		a.nNew++
    		// XXX time penalty?
    	}

    	bucket := a.getNewBucket(netAddr, srcAddr)

    	// Already exists?
    	if _, ok := a.addrNew[bucket][addr]; ok {
    		return
    	}

    	// Enforce max addresses.
    	if len(a.addrNew[bucket]) > newBucketSize {
    		log.Tracef("new bucket is full, expiring old")
    		a.expireNew(bucket)
    	}

    	// Add to new bucket.
    	ka.refs++
    	a.addrNew[bucket][addr] = ka

    	log.Tracef("Added new address %s for a total of %d addresses", addr,
    		a.nTried+a.nNew)
    }

来分析一下其主要步骤：

- 1.判断将要添加的地址netAddr是否是可路由的地址，即除了保留地址以外的地址，如果是不可以路由的地址，不加入到地址仓库。
    - if !IsRoutable(netAddr) {
- 2.查询将要添加的地址是否已经在地址集中，如果已经存在，但是其时间戳（节点最近获知该地址的时间点）更新或者有支持新的服务，更新地址集中KnownAddress。
    - if netAddr.Timestamp.After(ka.na.Timestamp) ||(ka.na.Services&netAddr.Services) != netAddr.Services
- 3.检查如果地址已经在TriedBucket中，则不更新地址仓库，如果地址已经位于8个不同的NewBucket中，也不更新仓库，根据地址已经被NewBucket引用的个数，来随机决定是否继续添加到NewBucket中。
    - if ka.tried
    - if ka.refs == newBucketsPerAddress
    - if a.rand.Int31n(factor) != 0
- 4.如果将要添加的地址不在现有的地址集中，则需要将其添加到NewBucket中。
    - a.addrIndex[addr] = ka
- 5.经过上述检查后，如果确定需要添加地址，则调用getNewBucket()找到NewBucket的索引。
    -  bucket := a.getNewBucket(netAddr, srcAddr)
- 6.确定了NewBucket的索引后，进一步检查将要添加的地址是否已经在对应的NewBucket时，如果是，则不再加入。
    - if _, ok := a.addrNew[bucket][addr]; ok
- 7.如果将要添加地址的NewBucket的Size超过newBucketSize，则调用expireNew()来释放Bucket里的一些记录。
    - len(a.addrNew[bucket]) > newBucketSize
    - a.expireNew(bucket)
    - 抛弃掉一部分地址，规则是时间戳最早或者时间戳是未来时间点的，或者时间戳是一个月以前，或者尝试连接失败次数超过3次且没有成功的地址，或者一周连接失败超过10次的地址移除。
- 8.最后，将新地址添加到NewBucket里。
    - a.addrNew[bucket][addr] = ka

上面方法有调用getNewBucket()来获取Bucket的索引：

    func (a *AddrManager) getNewBucket(netAddr, srcAddr *wire.NetAddress) int {
    	// bitcoind:
    	// doublesha256(key + sourcegroup + int64(doublesha256(key + group + sourcegroup))%bucket_per_source_group) % num_new_buckets

    	data1 := []byte{}
    	data1 = append(data1, a.key[:]...)
    	data1 = append(data1, []byte(GroupKey(netAddr))...)
    	data1 = append(data1, []byte(GroupKey(srcAddr))...)
    	hash1 := chainhash.DoubleHashB(data1)
    	hash64 := binary.LittleEndian.Uint64(hash1)
    	hash64 %= newBucketsPerGroup
    	var hashbuf [8]byte
    	binary.LittleEndian.PutUint64(hashbuf[:], hash64)
    	data2 := []byte{}
    	data2 = append(data2, a.key[:]...)
    	data2 = append(data2, GroupKey(srcAddr)...)
    	data2 = append(data2, hashbuf[:]...)

    	hash2 := chainhash.DoubleHashB(data2)
    	return int(binary.LittleEndian.Uint64(hash2) % newBucketCount)
    }

注释已经说得很清楚了，NewBucket的索引由AddrManager的随机序列key，地址newAddr已经通知该地址的Peer的地址
srcAddr共同决定。TriedBucket的索引也是采用类似的方式决定。

当有地址添加或者更新时，会在下一次dumpAddressTicker被写入到文件中，除了收到addr消息后，主动调用
AddAddress()或者AddAddresses()来更新地址集外，在节点选择地址并建立Peer关系成功后，也会调用Good()来
将地址NewBucket移入TriedBucket。

    func (a *AddrManager) Good(addr *wire.NetAddress) {
    	a.mtx.Lock()
    	defer a.mtx.Unlock()

    	ka := a.find(addr)
    	if ka == nil {
    		return
    	}

    	// ka.Timestamp在此处未更新，以避免泄漏有关当前连接的Peer的信息。
    	now := time.Now()
    	ka.lastsuccess = now
    	ka.lastattempt = now
    	ka.attempts = 0

    	// 移到tried地址集合，如果有需要，可以驱逐其他地址
    	if ka.tried {
    		return
    	}

    	// ok, need to move it to tried.

    	// remove from all new buckets. record one of the buckets in question and call it the `first'
    	addrKey := NetAddressKey(addr)
    	oldBucket := -1
    	for i := range a.addrNew {
    		// we check for existence so we can record the first one
    		if _, ok := a.addrNew[i][addrKey]; ok {
    			delete(a.addrNew[i], addrKey)
    			ka.refs--
    			if oldBucket == -1 {
    				oldBucket = i
    			}
    		}
    	}
    	a.nNew--

    	if oldBucket == -1 {
    		// What? wasn't in a bucket after all.... Panic?
    		return
    	}

    	bucket := a.getTriedBucket(ka.na)

    	// Room in this tried bucket?
    	if a.addrTried[bucket].Len() < triedBucketSize {
    		ka.tried = true
    		a.addrTried[bucket].PushBack(ka)
    		a.nTried++
    		return
    	}

    	// No room, we have to evict something else.
    	entry := a.pickTried(bucket)
    	rmka := entry.Value.(*KnownAddress)

    	// First bucket it would have been put in.
    	newBucket := a.getNewBucket(rmka.na, rmka.srcAddr)

    	// If no room in the original bucket, we put it in a bucket we just freed up a space in.
    	if len(a.addrNew[newBucket]) >= newBucketSize {
    		newBucket = oldBucket
    	}

    	// replace with ka in list.
    	ka.tried = true
    	entry.Value = ka

    	rmka.tried = false
    	rmka.refs++

    	// 我们在这里没有触及a.nTried，因为尝试的次数保持不变，但是我们拒绝了新的上面，因为我们放回一些东西而再次提高它。
    	a.nNew++

    	rmkey := NetAddressKey(rmka.na)
    	log.Tracef("Replacing %s with %s in tried", rmkey, addrKey)

    	// We made sure there is space here just above.
    	a.addrNew[newBucket][rmkey] = rmka
    }

其主要步骤如下：

- 1.查询连接成功的地址是否在地址集中，如果不在，则不作处理。
    - ka := a.find(addr)
- 2.如果地址在地址集中，则更新该地址的lastsuccess和lastattempt为当前时间点，且将连接重试次数的attempts重置。
    - now := time.Now()                                                            (2)
    - ka.lastsuccess = now
    - ka.lastattempt = now
    - ka.attempts = 0
- 3.如果地址已经在TriedBucket中，则只更新lastsuccess，lastattempt和attempts即可。
- 4.如果地址在NewBucket中，则将其从对应的Bucket中移除，记录下地址所处的NewBucket的索引号oldBucket。
    - if _, ok := a.addrNew[i][addrKey]; ok { delete(a.addrNew[i], addrKey)
    - if oldBucket == -1 {oldBucket = i
- 5.选择一个TriedBucket的索引号，用于将地址添加进对应的Bucket。
    -  bucket := a.getTriedBucket(ka.na)
- 6.如果选择的TriedBucket未填满，则将地址添加到Bucket。
    - a.addrTried[bucket].PushBack(ka)
- 7.如果选择的TriedBucket已经填满，则调用pickTried()从其中选择一个地址，准备将其移动到NewBucket中以腾出空间，随后为该地址选择一个NewBucket。
    - entry := a.pickTried(bucket)
    -  newBucket := a.getNewBucket(rmka.na, rmka.srcAddr)
- 8.如果想要移入的NewBucket已经满，则将选择的地址从TriedBucket中移入索引号为oldBucket的NewBucket中，即移入刚刚移除了addr的NewBucket中。
    - if len(a.addrNew[newBucket]) >= newBucketSize {newBucket = oldBucket
- 9.将连接成功的地址添加到选择的TriedBucket中，通过将listElement的Value直接更新为对应的ka来实现。
    - entry.Value = ka
- 10.将从TriedBucket中移出的地址移入选择的NewBucket中。
    - a.addrNew[newBucket][rmkey] = rmka

AddrManager通过GetAddress()来实现随机选取一个地址，让节点建立Peer连接：

    func (a *AddrManager) GetAddress() *KnownAddress {
    	// Protect concurrent access.
    	a.mtx.Lock()
    	defer a.mtx.Unlock()

    	if a.numAddresses() == 0 {
    		return nil
    	}

    	// Use a 50% chance for choosing between tried and new table entries.
    	if a.nTried > 0 && (a.nNew == 0 || a.rand.Intn(2) == 0) {
    		// Tried entry.
    		large := 1 << 30
    		factor := 1.0
    		for {
    			// pick a random bucket.
    			bucket := a.rand.Intn(len(a.addrTried))
    			if a.addrTried[bucket].Len() == 0 {
    				continue
    			}

    			// Pick a random entry in the list
    			e := a.addrTried[bucket].Front()
    			for i :=
    				a.rand.Int63n(int64(a.addrTried[bucket].Len())); i > 0; i-- {
    				e = e.Next()
    			}
    			ka := e.Value.(*KnownAddress)
    			randval := a.rand.Intn(large)
    			if float64(randval) < (factor * ka.chance() * float64(large)) {
    				log.Tracef("Selected %v from tried bucket",
    					NetAddressKey(ka.na))
    				return ka
    			}
    			factor *= 1.2
    		}
    	} else {
    		// new node.
    		// XXX use a closure/function to avoid repeating this.
    		large := 1 << 30
    		factor := 1.0
    		for {
    			// Pick a random bucket.
    			bucket := a.rand.Intn(len(a.addrNew))
    			if len(a.addrNew[bucket]) == 0 {
    				continue
    			}
    			// Then, a random entry in it.
    			var ka *KnownAddress
    			nth := a.rand.Intn(len(a.addrNew[bucket]))
    			for _, value := range a.addrNew[bucket] {
    				if nth == 0 {
    					ka = value
    				}
    				nth--
    			}
    			randval := a.rand.Intn(large)
    			if float64(randval) < (factor * ka.chance() * float64(large)) {
    				log.Tracef("Selected %v from new bucket",
    					NetAddressKey(ka.na))
    				return ka
    			}
    			factor *= 1.2
    		}
    	}
    }

其主要步骤为：

- 1.如地址集中的NewBucket和TriedBucket，既有已经尝试连接过的老地址，也有未连接过的新地址，即按50%的概率随机从两种地址集中选择。
    - if a.nTried > 0 && (a.nNew == 0 || a.rand.Intn(2) == 0) {
- 2.如果决定从TriedBucket中选择，则随机选择一个TriedBucket。
    -  bucket := a.rand.Intn(len(a.addrTried))
- 3.从随机选择的TriedBucket中，再随机选择一个地址。
    - a.rand.Int63n(int64(a.addrTried[bucket].Len())); i > 0; i--
- 4.判断是否满足随机条件
    - if float64(randval) < (factor * ka.chance() * float64(large))
    - factor *= 1.2  //如果不满足，增增加factor因子
    - 不满足的话，继续重复2-4的步骤，直到满足随机条件
- 5.如果决定从NewBucket中选择，则采取与TriedBucket相似的步骤随机选择地址。

从上可以看出，这个随机条件为：从0到一个很大的数（large := 1 << 30）的范围内选择一个数，然后看这个数是否小于它本身
乘以factor再乘以ka.chance()。可以看到，ka.chance()越大，该条件成立的概率越大。

    func (ka *KnownAddress) chance() float64 {
    	now := time.Now()
    	lastAttempt := now.Sub(ka.lastattempt)

    	if lastAttempt < 0 {
    		lastAttempt = 0
    	}

    	c := 1.0

    	// Very recent attempts are less likely to be retried.
    	if lastAttempt < 10*time.Minute {
    		c *= 0.01
    	}

    	// Failed attempts deprioritise.
    	for i := ka.attempts; i > 0; i-- {
    		c /= 1.5
    	}

    	return c
    }

可以看到，如果十分钟之内尝试连接过，地址的选择概率将降为1%，同时，每尝试失败一次，则被选中的概率降为原来的2/3。
    - if lastAttempt < 10*time.Minute {c *= 0.01}
    - for i := ka.attempts; i > 0; i-- {c /= 1.5}

也就是说，如果10分钟之内尝试连接失败过，或者多次连接失败，则该地址被选中的概率大大降低。

这个addrmanager的内容不是很多，主要负责将从Peer学习到的地址分为新地址和老地址，并通过NewBucket和TriedBucket来管理。
同时周期性地将地址集写入文件存储。最重要的，需要从地址集中随机选择地址的策略，使得节点可以随机选择Peer，从而避免恶意节点的钓鱼攻击。











