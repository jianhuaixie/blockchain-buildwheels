# BTCD区块协议消息

BTCD通过AddrManager进行Peer地址管理，ConnManager进行Peer连接管理，这些都是P2P网络建立的基础，其过程：先通过AddrManager
选择Peer地址，并通过ConnManager建立TCP连接，然后通过Peer开始收发协议消息。之前有介绍过Peer节点会交换getaddr和addr消息来同步地址信息，
除此之外，还会交换区块信息，就需要定义一个区块协议来标准化各个Peer之间能读懂的区块信息。

Bitcoin节点将Transaction和Block在全网广播，就是通过在Peer与Peer之间同步Transaction和Block实现，为了新建和维持Peer关系，
协议也定义了ping/pong心跳和getaddr/addr等消息。协议消息的定义在btcd/wire包中实现，wire只定义了消息格式，消息的封装和解析方法等，
也就是说，wire只知道消息是什么样子的，并不知道消息是如何收发的，没有定义协议交互，不包含收到消息后如何处理和响应的逻辑。

消息的响应和交互在ServerPeer中实现，其中涉及到的区块处理的逻辑在blockmanager和btcd/blockchain中实现。

btcd/wire主要包含如下文件：

- protocol.go:定义了Bitcoin协议的版本好，网络号及ServiceFlag等常量。
- common.go:定义了向二进制流读写基础数据类型的方法，同时也定义了读写[可变长度整数值](https://www.jianshu.com/p/a52c16fca39e)和可变长度字符串的方法。
- message.go:定义了Message接口及消息封装和解析的工厂方法。
- msgXXX.go:定义了具体消息的格式及接口方法的实现。
- blockheader.go:定义了BlockHeader类型，用于block，headers等消息。
- invvect.go:定义了InvVect类型，用于inv消息。
- netaddress.go:定义了NetAddress类型。

为了学习消息协议，可以先阅读bitcoinwiki中文版本 [协议说明](https://zh-cn.bitcoin.it/wiki/%E5%8D%8F%E8%AE%AE%E8%AF%B4%E6%98%8E)来对协议消息的格式全面了解。

首先看看在message.go中的Meesage接口和消息结构定义：

    type Message interface {
    	BtcDecode(io.Reader, uint32, MessageEncoding) error
    	BtcEncode(io.Writer, uint32, MessageEncoding) error
    	Command() string
    	MaxPayloadLength(uint32) uint32
    }

    type messageHeader struct {
    	magic    BitcoinNet // 4 bytes 标识Bitcoin协议消息的“魔数”，同时也用于区分Bitcoin网络，节点启动时可以指定在哪个网络下运行
    	command  string     // 12 bytes 协议消息包含一个命令字符串，如version，addr等，用于标识协议消息的类型
    	length   uint32     // 4 bytes 消息体的长度
    	checksum [4]byte    // 4 bytes 消息体头4个字节的双SHA256的结果
    }

协议消息的结构如下图所示：

![](http://chuantu.biz/t6/330/1529391745x-1376440210.png)

Message接口中用BtcDecode()和BtcEncode()定义了解析和封装消息体的方法，他们会在每个具体的消息定义中继承接口来实现，
当然这里的decode和encode主要是将结构化的消息体序列化为字节流或将字节流实例化为某种消息格式，其中需要用到common.go中定义
的各种读写基础数据类型的方法。

先看看common.go中定义的各个方法：

    func readElement(r io.Reader, element interface{}) error {
    	// Attempt to read the element based on the concrete type via fast
    	// type assertions first.
    	switch e := element.(type) {
    	case *int32:
    		rv, err := binarySerializer.Uint32(r, littleEndian)
    		if err != nil {
    			return err
    		}
    		*e = int32(rv)
    		return nil

    	......

    	case *bool:
    		rv, err := binarySerializer.Uint8(r)
    		if err != nil {
    			return err
    		}
    		if rv == 0x00 {
    			*e = false
    		} else {
    			*e = true
    		}
    		return nil

    	......

    	// Message header checksum.
    	case *[4]byte:
    		_, err := io.ReadFull(r, e[:])
    		if err != nil {
    			return err
    		}
    		return nil

    	// Message header command.
    	case *[CommandSize]uint8:
    		_, err := io.ReadFull(r, e[:])
    		if err != nil {
    			return err
    		}
    		return nil

    	......

    	// Fall back to the slower binary.Read if a fast path was not available above.
    	return binary.Read(r, littleEndian, element)
    }

readElement()方法其主要过程是通过类型断言（type assertion）解析欲读取字节对应的数据类型，然后根据类型的
size读出字节slice，并通过强制类型转换后得到格式化的数据。writeElement()则是与其完全相反的过程。

rv, err := binarySerializer.Uint64(r, littleEndian)

对于uint8，uint32和uint64等基础数据类型的读写是通过binarySerializer的读写方法。binarySerializer是一个缓冲为1024
个字节，容量为8字节的byte slice管道，是用作一个缓存队列使用，为了避免序列化和反序列化基础数据类型时频繁地分配和释放内存，
binarySerializer提供了一个大小固定的缓存池，当需要缓存时，直接向缓存池借指定大小的byte slice，使用完毕后归还内存。

当然，当固定大小的缓存池被分配完毕后，后续的申请并不会被阻塞，而是直接从内存分配，使用完毕后会给gc回收。

    type binaryFreeList chan []byte

    func (l binaryFreeList) Borrow() []byte {
    	var buf []byte
    	select {
    	case buf = <-l:
    	default:
    		buf = make([]byte, 8)
    	}
    	return buf[:8]
    }

    func (l binaryFreeList) Return(buf []byte) {
    	select {
    	case l <- buf:
    	default:
    		// Let it go to the garbage collector.
    	}
    }

    // binarySerializer provides a free list of buffers to use for serializing and
    // deserializing primitive integer values to and from io.Readers and io.Writers.
    var binarySerializer binaryFreeList = make(chan []byte, binaryFreeListMaxItems)

上面的Borrow()和Return()方法中，select复用均添加了default分支，因此在缓存池空时申请或者缓存池满时释放均不会阻塞。

从缓存池中申请的byte slice的容量为8字节，即最大可以支持uint64类型的缓存。slice管道是一个非常简洁的方式来实现
缓存池，slice不同于数组，它指向底层数组，所以Borrow()和Return()的入参和返回值并没有发生数组拷贝。

为了了解一下基础数据类型的序列化过程，我们可以看binaryFreeList的PutUint16()：

    // PutUint8 copies the provided uint8 into a buffer from the free list and
    // writes the resulting byte to the given writer.
    func (l binaryFreeList) PutUint8(w io.Writer, val uint8) error {
    	buf := l.Borrow()[:1]
    	buf[0] = val
    	_, err := w.Write(buf)
    	l.Return(buf)
    	return err
    }

除了基础数据类型，为了压缩传输数据量，bitcoin协议定义了可变长度整数值，对于此概念，可以阅读开头那篇关于可变长度整数值的文章。

通过可变长度整数值的序列化方法WriteVarInt()来理解：

    // WriteVarInt serializes val to w using a variable number of bytes depending on its value.
    func WriteVarInt(w io.Writer, pver uint32, val uint64) error {
    	if val < 0xfd {
    		return binarySerializer.PutUint8(w, uint8(val))
    	}

    	if val <= math.MaxUint16 {
    		err := binarySerializer.PutUint8(w, 0xfd)
    		if err != nil {
    			return err
    		}
    		return binarySerializer.PutUint16(w, littleEndian, uint16(val))
    	}

    	if val <= math.MaxUint32 {
    		err := binarySerializer.PutUint8(w, 0xfe)
    		if err != nil {
    			return err
    		}
    		return binarySerializer.PutUint32(w, littleEndian, uint32(val))
    	}

    	err := binarySerializer.PutUint8(w, 0xff)
    	if err != nil {
    		return err
    	}
    	return binarySerializer.PutUint64(w, littleEndian, val)
    }

WriteVarInt()完全按照可变长度整数值的定义，根据整数值的大小范围将其编码成不同长度的字节序列。

ReadVarInt()则是完全相反的过程。除了可变长度整数值，还有可变长度字符串和可变长度字节流，
均是通过在字符串或者字节流头部添加一个表示实际长度的可变长度整数数值来实现的。

以上是基础数据类型的读写，基于此，会有Message的封装和解析，对应到WriteMessage()和ReadMesssage()方法。

- _, msg, buf, err := ReadMessageN(r, pver, btcnet)
- return ReadMessageWithEncodingN(r, pver, btcnet, BaseEncoding)

    func ReadMessageWithEncodingN(r io.Reader, pver uint32, btcnet BitcoinNet,
    	enc MessageEncoding) (int, Message, []byte, error) {

    	totalBytes := 0
    	n, hdr, err := readMessageHeader(r)
    	totalBytes += n
    	if err != nil {
    		return totalBytes, nil, nil, err
    	}

    	// Enforce maximum message payload.
    	if hdr.length > MaxMessagePayload {
    		str := fmt.Sprintf("message payload is too large - header "+
    			"indicates %d bytes, but max message payload is %d "+
    			"bytes.", hdr.length, MaxMessagePayload)
    		return totalBytes, nil, nil, messageError("ReadMessage", str)

    	}

    	// Check for messages from the wrong bitcoin network.
    	if hdr.magic != btcnet {
    		discardInput(r, hdr.length)
    		str := fmt.Sprintf("message from other network [%v]", hdr.magic)
    		return totalBytes, nil, nil, messageError("ReadMessage", str)
    	}

    	// Check for malformed commands.
    	command := hdr.command
    	if !utf8.ValidString(command) {
    		discardInput(r, hdr.length)
    		str := fmt.Sprintf("invalid command %v", []byte(command))
    		return totalBytes, nil, nil, messageError("ReadMessage", str)
    	}

    	// Create struct of appropriate message type based on the command.
    	msg, err := makeEmptyMessage(command)
    	if err != nil {
    		discardInput(r, hdr.length)
    		return totalBytes, nil, nil, messageError("ReadMessage",
    			err.Error())
    	}

    	// Check for maximum length based on the message type as a malicious client
    	// could otherwise create a well-formed header and set the length to max
    	// numbers in order to exhaust the machine's memory.
    	mpl := msg.MaxPayloadLength(pver)
    	if hdr.length > mpl {
    		discardInput(r, hdr.length)
    		str := fmt.Sprintf("payload exceeds max length - header "+
    			"indicates %v bytes, but max payload size for "+
    			"messages of type [%v] is %v.", hdr.length, command, mpl)
    		return totalBytes, nil, nil, messageError("ReadMessage", str)
    	}

    	// Read payload.
    	payload := make([]byte, hdr.length)
    	n, err = io.ReadFull(r, payload)
    	totalBytes += n
    	if err != nil {
    		return totalBytes, nil, nil, err
    	}

    	// Test checksum.
    	checksum := chainhash.DoubleHashB(payload)[0:4]
    	if !bytes.Equal(checksum[:], hdr.checksum[:]) {
    		str := fmt.Sprintf("payload checksum failed - header "+
    			"indicates %v, but actual checksum is %v.",
    			hdr.checksum, checksum)
    		return totalBytes, nil, nil, messageError("ReadMessage", str)
    	}

    	// Unmarshal message.  NOTE: This must be a *bytes.Buffer since the
    	// MsgVersion BtcDecode function requires it.
    	pr := bytes.NewBuffer(payload)
    	err = msg.BtcDecode(pr, pver, enc)
    	if err != nil {
    		return totalBytes, nil, nil, err
    	}

    	return totalBytes, msg, payload, nil
    }

其主要过程为：

- 1.读取并解析消息头。
    - n, hdr, err := readMessageHeader(r)
    - r为io.Reader实际为net.Conn对象，也就是读TCP Socket
- 2.检查头部里填的payload长度是否超过32M的限制，如果超过限制，表明它可能是一个恶意包，停止解析并返回错误。
    -  if hdr.length > MaxMessagePayload {
- 3.检测头部里的magic number，如果不是从指定的网络上收到的包，则丢弃。
    - if hdr.magic != btcnet {
- 4.检测头部里的command字段，如果不是包含非utf8字符，则丢弃该包。
    -  if !utf8.ValidString(command) {
- 5.根据command来构造空的对应类型的消息，为解析payload作准备。
    - msg, err := makeEmptyMessage(command)
- 6.在读取payload之间，进一步检测头部里声明的payload长度是否超过了对应消息规定的最大长度，如果超过则丢弃。
    -  if hdr.length > mpl {
- 7.检查完毕后，读取payload，在最后解析payload之前，对payload进行hash校验，检查payload是否被篡改过。
    - n, err = io.ReadFull(r, payload)
    - checksum := chainhash.DoubleHashB(payload)[0:4]
- 8.最后，调用Message的抽象方法BtcDecode()对消息体进行解析，如果解析正确，则返回解析的结果。
    -  err = msg.BtcDecode(pr, pver)
    -  return totalBytes, msg, payload, nil

消息的解析过程依赖具体消息类型的BtcDecode()和MaxPayloadLength()实现。
消息的封装过程依赖BtcEncode()和Command()实现。

具体消息类型，Bitcoin协议定义了约27种消息，核心的有version，verack，add，inv，getdata，notfound，
getblocks，getheaders，tx，block，headers，getaddr，ping和pong等14种，首先绘制一个核心消息的交互图。

![](http://chuantu.biz/t6/330/1529395551x-1376440210.png)

图中绿色和红色表示不同的通信方向，一组绿色和红色构成一次消息交互。Peer之间通过交换version和verack来协商版本号。
通过ping和pong来维持Peer连接。
Peer之间通过getaddr和addr来同步地址仓库。
Peer之间通过getblocks-inv-getdata-block|tx交互来同步transaction或者block，并进一步达成区块链的共识。
一些Bitcoin轻量级客户端不希望下载区块链上完整的交易记录，可以只下载区块的头部，通过getheaders和headers消息来同步区块头。

当节点收到新的transaction或者block后，可以通过inv消息主动向Peer通知新的tx或者block。

alter消息用于核心节点向全网发布通告，目前已经弃用。

到目前，还剩下version,inv,getblocks,getdata,block,tx等消息交互没有深入分析。

###### version

version消息的格式定义如下：

    type MsgVersion struct {
    	// Version of the protocol the node is using.
    	ProtocolVersion int32

    	// Bitfield which identifies the enabled services.
    	Services ServiceFlag

    	// Time the message was generated.  This is encoded as an int64 on the wire.
    	Timestamp time.Time

    	// Address of the remote peer.
    	AddrYou NetAddress

    	// Address of the local peer.
    	AddrMe NetAddress

    	// Unique value associated with message that is used to detect self
    	// connections.
    	Nonce uint64

    	// The user agent that generated messsage.  This is a encoded as a varString
    	// on the wire.  This has a max length of MaxUserAgentLen.
    	UserAgent string

    	// Last block seen by the generator of the version message.
    	LastBlock int32

    	// Don't announce transactions to peer.
    	DisableRelayTx bool
    }

MsgVersion中各个属性的定义比较简单，其中有一个NetAddress：

    type NetAddress struct {
    	// 记录节点从外部获知该地址的最近时间点，如果此时间点越早，也就是该地址
    	// 存活期越长，对应地址失效的可能性就越大。version消息里的发送端（AddrMe）
    	// 和接收端地址（AddrYou）没有包含该字段。
    	Timestamp time.Time

    	// 表明节点支持的服务，即节点类型，包含SFNodeNetwork
    	Services ServiceFlag

    	// IP address of the peer.
    	IP net.IP

    	// Port the peer is using.  This is encoded in big endian on the wire
    	// which differs from most everything else.
    	Port uint16
    }

熟悉version的格式定义后，理解BtcEncode()和BtcDecode()变得非常简单，无非就是调用writeElement()
或者readElement()等方法对不同的数据类型进行读写。

MessageVersion的BtcEncode()和BtcDecode()比较简单。

###### inv

inv消息的格式定义如下：

    // Use the AddInvVect function to build up the list of inventory vectors when
    // sending an inv message to another peer.
    type MsgInv struct {
    	InvList []*InvVect
    }

inv主要用来向Peer通知区块或者交易数据，它是getblocks消息的响应消息，也可以主动发送。inv消息体包含一个
InvVect列表和表示InvVect个数的可变长度整数Count值。

InvVect的定义格式：

    type InvVect struct {
    	Type InvType        // 指定数据的类型，如Tx，Block或者FilteredBlock
    	Hash chainhash.Hash // 对应数据的Hash值，如某个transaction的hash或者block头的hash
    }

    const (
    	InvTypeError                InvType = 0
    	InvTypeTx                   InvType = 1
    	InvTypeBlock                InvType = 2
    	InvTypeFilteredBlock        InvType = 3
    	InvTypeWitnessBlock         InvType = InvTypeBlock | InvWitnessFlag
    	InvTypeWitnessTx            InvType = InvTypeTx | InvWitnessFlag
    	InvTypeFilteredWitnessBlock InvType = InvTypeFilteredBlock | InvWitnessFlag
    )

###### getblocks

    type MsgGetBlocks struct {
    	ProtocolVersion    uint32               //协议的版本号
    	BlockLocatorHashes []*chainhash.Hash    //记录一个BlockLocator，BlockLocator用于定位列表中第一个block元素在区块链中的位置
    	HashStop           chainhash.Hash       //getblocks请求的block区间的结束位置
    }

getblocks请求的区块位于BlockLocator指向的区块和HashStop指向的区块之间，不包括BlockLocator指向的区块。
如果HashStop为零，则返回BlockLocator指向的区块之后的500个区块。当然需要理解BlockLocator：

在/btcd/blockchain/chain.go中有BlockLocator的定义：

    type BlockLocator []*chainhash.Hash

BlockLocator实际是一个*chainhash.Hash类型的slice，用于记录一组block的hash值，slice中的第一个元素即BlockLocator指向的区块。

区块链可能分叉，为了致命该区块的位置，BlockLocator记录了从指定区块回溯到创世区块的路径。

BlockLocator中的前10个hash值是一个接着一个的区块hash值，第11个元素后步长成级数增加，即每一次
向前回溯，步长翻倍，加速向创世区块回溯，保证了BlockLocator中元素并不是很多。

最重要的，BlockLocator记录slice中第一个元素代表区块的位置。

通过分析MsgGetBlocks的BtcEncode()方法来了解getblocks消息体的格式：

    func (msg *MsgGetBlocks) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
    	count := len(msg.BlockLocatorHashes)
    	if count > MaxBlockLocatorsPerMsg {
    		str := fmt.Sprintf("too many block locator hashes for message "+
    			"[count %v, max %v]", count, MaxBlockLocatorsPerMsg)
    		return messageError("MsgGetBlocks.BtcEncode", str)
    	}

    	err := writeElement(w, msg.ProtocolVersion)
    	if err != nil {
    		return err
    	}

    	err = WriteVarInt(w, pver, uint64(count))
    	if err != nil {
    		return err
    	}

    	for _, hash := range msg.BlockLocatorHashes {
    		err = writeElement(w, hash)
    		if err != nil {
    			return err
    		}
    	}

    	return writeElement(w, &msg.HashStop)
    }

MsgGetBlocks序列化时按顺序写入协议版本号，BlockLocator中hash个数，BlockLocator中hash列表及截至hash值。
这就是getblocks消息体的格式。

###### getdata

getdata的消息定义：

    type MsgGetData struct {
    	InvList []*InvVect
    }

节点收到Peer的inv通告后，发现有更新的区块或者交易，则可以向Peer发送getdata请求来同步区块或者交易。

getdata消息比较简单，与inv类似，它的消息体包含了InvVect列表，指明自己希望同步的区块或者交易的hash列表。

Peer收到后回复block或tx消息，将区块或者交易发送给节点。

###### tx

tx消息用于在Peer之间同步transactions：

    type MsgTx struct {
    	Version  int32      //Tx的版本号，
    	TxIn     []*TxIn    //引用的输入交易的UTXO(s),包含上一个交易的hash值和index
    	TxOut    []*TxOut   //当前交易的输出UTXO(s)
    	LockTime uint32     //既可以表示UTC时间，也可以表示区块高度。
    }

    type TxIn struct {
    	PreviousOutPoint OutPoint   //其中的Index即是前一个交易的[]*TxOut中的索引号
    	SignatureScript  []byte     //解锁脚本
    	Witness          TxWitness  //TxWitness定义TxIn的见证。见证者将被解释为一片字节片，或者是一个或多个元素的堆栈
    	Sequence         uint32     //输入交易的序号，对于同一个交易，矿工优先选择Sequence更大的交易加入区块进行挖矿
    }

    type TxOut struct {
    	Value    int64      //bitcoin数量，单位是聪
    	PkScript []byte     //解锁脚本
    }

    // OutPoint定义了一个用于追踪以前交易输出的比特币数据类型.
    type OutPoint struct {
    	Hash  chainhash.Hash    //上一个交易的hash值
    	Index uint32            //表示上一个交易的输出的序号
    }

从MsgTx的定义可以看出，一个transaction中主要包含一个TxIn的列表和TxOut列表。

TxIn实际上指向输入交易的UTXO，TxOut是当前交易的UTXO。

从PreviousOutPoint的定义中，可以看到所有的交易均会向前引用形成一条“交易链”，直到coinbase交易。

我们看/btcd/wire/msgtx.go中的TxHash()方法了解交易的Hash是如何计算的：

    // TxHash generates the Hash for the transaction.
    func (msg *MsgTx) TxHash() chainhash.Hash {
    	// Encode the transaction and calculate double sha256 on the result.
    	// Ignore the error returns since the only way the encode could fail
    	// is being out of memory or due to nil pointers, both of which would
    	// cause a run-time panic.
    	buf := bytes.NewBuffer(make([]byte, 0, msg.SerializeSizeStripped()))
    	_ = msg.SerializeNoWitness(buf)
    	return chainhash.DoubleHashH(buf.Bytes())
    }

交易的Hash是整个交易结构的字节流进行两次SHA256()后的结果。其中Serialize()方法就是调用
BtcEncode()对MsgTx进行序列化。

    func (msg *MsgTx) SerializeNoWitness(w io.Writer) error {
    	return msg.BtcEncode(w, 0, BaseEncoding)
    }

BtcEncode()或BtcDecode()就是按MsgTx的定义逐元素写或者读。

###### block

除了tx外，block是btcd/wire里最重要的概念，定义了区块的结构：

    type MsgBlock struct {
    	Header       BlockHeader
    	Transactions []*MsgTx
    }

区块里包含区块头和一些列交易的集合，区块头的定义为：

    type BlockHeader struct {
    	// 区块的版本，与协议版本号不同
    	Version int32

    	// 链上前一个区块的Hash值，每个区块都通过该字段指向上一个区块，直到创世区块，从而形成链结构
    	PrevBlock chainhash.Hash

    	// Merkle树的树根Hash，包含了区块中所有交易的信息的hash值。
    	MerkleRoot chainhash.Hash

    	// 区块创建的时间点
    	Timestamp time.Time

    	// 挖矿的难度
    	Bits uint32

    	// 用于挖矿或验证区块难度的随机值
    	Nonce uint32
    }

通过MsgBlock的BtcEncode()方法可以了解区块序列化后的格式：

    func (msg *MsgBlock) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
    	err := writeBlockHeader(w, pver, &msg.Header)
    	if err != nil {
    		return err
    	}

    	err = WriteVarInt(w, pver, uint64(len(msg.Transactions)))
    	if err != nil {
    		return err
    	}

    	for _, tx := range msg.Transactions {
    		err = tx.BtcEncode(w, pver, enc)
    		if err != nil {
    			return err
    		}
    	}

    	return nil
    }

区块的序列化结构中包含区块头，表示交易数量的整数值和交易列表。

其结构如下图：

![](http://chuantu.biz/t6/330/1529400774x-1376440210.png)

区块头是不包含交易数量值的，在计算区块的Hash值时，由于MerkleRoot已经包涵了所有交易的信息Hash，所以不用计算每个区块的Hash值，
只计算区块头的Hash，且不包括交易数量值。

    func (msg *MsgBlock) BlockHash() chainhash.Hash {
    	return msg.Header.BlockHash()
    }

这篇文章介绍了wire中协议消息封装和解析的过程，节点同步到Peer的transaction或者block后的处理，
或者节点收到Peer的getblocks或者getdata请求后如何从自己的交易池或者区块链上找到对方需要的交易或者区块呢

这些交互的逻辑将在接下来的文章中继续分析。

本文有借鉴oceanken的文章，在此表示感谢，他的博客地址：

https://www.jianshu.com/p/a0a54afe11c6
















