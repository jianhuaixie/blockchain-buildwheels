# 网络

之前版本的链具有的特性：匿名，安全，随机生成的地址；区块链数据存储；工作量证明机制；可靠地存储交易。

然后区块链的核心就是去中心化，没有网络进行节点通信，谈不上去中心化。

### 区块链网络

区块链网络是去中心化的，没有中心服务器，客户端也不需要依赖服务器来获取或处理数据。在区块链网络中，有的只是节点，
每个节点是网络的一个完全（full-fledged）成员。节点就是一切：它既是一个客户端，也是一个服务器。

区块链网络是一个P2P（peer-to-peer，端到端）的网络，即节点直接连接到其他节点。它的拓扑结构是扁平的，节点的世界中没有层级之分。

要实现这样一个网络节点更加困难，必须执行很多操作。每个节点必须与很多其他节点进行交互，必须请求其他节点的状态，与自己的状态
进行比较，当状态过时时进行更新。

### 节点角色

尽管节点具备完备成熟的属性，但它们可以在网络中扮演不同角色。比如：

- 1.矿工    这样的节点运行在强大或专用的硬件（ASIC芯片）上，目标是尽可能快的挖出新块。
- 2.全节点   这些节点验证矿工挖出来的块的有效性，并对交易进行确认。为此，需要拥有区块链的完整拷贝。同时，全节点执行路由操作，
帮助其他节点发现彼此。全节点要足够多，这些节点执行决策功能，决定了一个块或一笔交易的有效性。
- 3.SPV SPV表示Simplified Payment Verification，简单支付验证。这些节点并不存储整个区块链副本，但是仍能够对交易进行验证。
一个SPV节点依赖一个全节点来获取数据，SPV使得钱包应用成为可能：一个人不需要下载整个区块链，但是仍能够验证他的交易。

### 网络简化

目前这个版本的轮子，我们将重点放在区块链实现上，所以我们在一台机器上模拟出众多区块链节点。我们使用端口号作为节点标识符，而不是IP地址。
使用环境变量NODE_ID对这些端口节点ID进行设置。从而，可以打开多个终端端口，设置不同的NODE_ID运行不同的节点。

这个版本中所用的方法，也需要不同的区块链和钱包文件，当然可以简单依赖于节点ID进行命名，比如blockchain_3000.db,blockchain_3001.db,
wallet_3000.db,wallet_3001.db等。

### 实现

从比特币Bitcoin Core分析，首次下载时，必须连接到某个节点下载最新状态的区块链。但是实际上主机并没有意识到所有或者部分的比特币节点，
那么连接到的“某个节点”到底是什么？

如果直接硬编码一个节点地址，新加入的节点都从这个节点下载最新的区块链，这是有可能被DDOS等攻击的，导致新的节点根本无法加入进来。
在Bitcoin Core中，硬编码一个DNS seeds，这些并不是节点，但是DNS服务器知道一些节点的地址，当你启动一个全新的Bitcoin Core时，
它会连接到一个种子节点，获取全节点列表，随后从这些节点中下载区块链。

在这个版本的轮子中，无法做到完全的去中心化，我们会有三个节点：

- 1.一个中心节点。所有其他节点都会连接到这个节点，这个节点会和其他节点之间发送数据。
- 2.一个矿工节点。这个节点会在内存中存储新的交易，当有足够的交易时，就会打包挖出一个新块。
- 3.一个钱包节点。这个节点会被用作在钱包之间发送币。但是和SPV节点不同，它存储了区块链的一个完整副本。

### 场景

本版本的轮子能实现的场景：

- 1.中心节点创建一个区块链
- 2.一个其他（钱包）节点连接到中心节点并下载区块链
- 3.另一个（矿工）节点连接到中心节点并下载区块链
- 4.钱包节点创建一笔交易
- 5.矿工节点接收交易，将交易保存到内存池中
- 6.当内存池中有足够的交易时，矿工开始打包挖矿
- 7.当挖出一个新块后，将其发送到中心节点
- 8.钱包节点与中心节点进行同步
- 9.钱包节点的用户检查他们的支付是否成功

### 版本

节点通过消息（message）进行交流。当一个新的节点开始运行时，就会从一个DNS种子获取几个节点，
给它们发送version信息，实现起来如下：

    type version struct{
        Version int
        BestHeight  int
        AddrFrom    string
    }

Version存储区块链版本，BestHeight存储区块链中节点的高度。AddFrom存储发送者的地址。

节点在接收到version消息后，会响应自己的version消息。这是一种握手：如果没有事先问候，就不可能有其他交流。
version用于找到一个更长的区块链，当一个节点接收到的version消息，会检查本节点的区块链是否比BestHeight的值要大，
如果不是，节点就会请求并下载缺失的块。

为了接收消息，需要构建一个服务器：

    var nodeAddress string
    var knownNodes = []string{"localhost:3000"}
    func StartServer(nodeID,minerAddress string){
        nodeAddress = fmt.Sprintf("localhost:%s",nodeID)
        miningAddress = minerAddress
        ln,err := net.Listen(protocol,nodeAddress)
        defer ln.Close()
        blockchain := NewBlockchain(nodeID)
        // 对中心节点硬编码，当前节点不是中心节点，必须向中心节点发送version消息来查询是否自己的区块链已经过时
        if nodeAddress != knownNodes[0]{
            sendVersion(knownNodes[0],blockchain)
        }
        for {
            conn,err := ln.Accept()  // 一个节点不断接收命令，然后给下面的处理器选择函数来处理命令主体
            go handleConnection(conn,blockchain) // 选择正确的处理器处理命令主体
        }
    }

请求都是要进行解码，提取有效信息。所有的处理器在这部分都类似。然后节点将从消息中提取的BestHeight与自身进行比较。如果自身
节点的区块链更长，就会恢复version消息，否则，会发送getblocks消息

#### getblocks

    type getblocks struct{
        AddrFrom    string
    }

getblocks并不是说把所有区块都给我，而是请求一个块哈希的列表。也就是说，给我一个消息：你有哪些区块。我知道了有那些区块，我再
从其他节点分布式下载过来。

在简化的版本中，可以直接返回所有的哈希块。

#### inv

    type inv struct{
        AddrFrom    string
        Type    string
        Items   [][]byte
    }

比特币使用inv来向其他节点展示当前节点有什么块和交易。并没有包含完整的区块链和交易，仅仅是哈希而已。
Type表明这是块还是交易。

    func handleInv(request []byte, bc *Blockchain) {
        ...
        fmt.Printf("Recevied inventory with %d %s\n", len(payload.Items), payload.Type)

        if payload.Type == "block" {
            blocksInTransit = payload.Items

            blockHash := payload.Items[0]
            sendGetData(payload.AddrFrom, "block", blockHash)

            newInTransit := [][]byte{}
            for _, b := range blocksInTransit {
                if bytes.Compare(b, blockHash) != 0 {
                    newInTransit = append(newInTransit, b)
                }
            }
            blocksInTransit = newInTransit
        }

        if payload.Type == "tx" {
            txID := payload.Items[0]

            if mempool[hex.EncodeToString(txID)].ID == nil {
                sendGetData(payload.AddrFrom, "tx", txID)
            }
        }
    }

在收到哈希块，需要保存在blocksInTransit变量来跟踪已下载的块。能够让我们从不同的节点下载块。在将块置于传送状态时，
我们给inv消息的发送者发送getdata命令并更新blocksInTransit。在一个真实的P2P网络中，我们会想要从不同节点来传送块。

#### getdata

    type getdata struct{
        AddrFrom    string
        Type    string
        ID  []byte
    }

getdata用于某个块或交易的请求获取，可以仅包含一个块或交易ID。

    func handleGetData(request []byte, bc *Blockchain) {
        ...
        if payload.Type == "block" {
            block, err := bc.GetBlock([]byte(payload.ID))

            sendBlock(payload.AddrFrom, &block)
        }

        if payload.Type == "tx" {
            txID := hex.EncodeToString(payload.ID)
            tx := mempool[txID]

            sendTx(payload.AddrFrom, &tx)
        }
    }

上面的处理器比较直观：如果请求一个块，就返回块，如果请求一笔交易，则返回交易。实际上，还是需要检查是否已经有合格块和交易。

#### block和tx

    type block struct{
        AddrFrom    string
        Block   []byte
    }

    type tx struct{
        AddrFrom    string
        Transaction []byte
    }

实际完成数据转移的正是上面的这些消息。

    func handleBlock(request []byte, bc *Blockchain) {
        ...

        blockData := payload.Block
        block := DeserializeBlock(blockData)

        fmt.Println("Recevied a new block!")
        bc.AddBlock(block)

        fmt.Printf("Added block %x\n", block.Hash)

        if len(blocksInTransit) > 0 {
            blockHash := blocksInTransit[0]
            sendGetData(payload.AddrFrom, "block", blockHash)

            blocksInTransit = blocksInTransit[1:]
        } else {
            UTXOSet := UTXOSet{bc}
            UTXOSet.Reindex()
        }
    }

处理块的命令函数，当接收到一个新块时，放到区块链里面。

如果还有块要继续下载，就还是从那个节点继续请求。当下载好，就对UTXO集进行重新索引。

> todo:并非无条件信任，应该在将每个块加入到区块链之前进行验证
> todo:并非与西宁UTXOSet.Reindex()，而是应该使用UTXOSet.Update(block)，因为如果区块链很大，需要很多时间来对整个UTXO集重新索引。

处理tx消息是最困难的部分：

    func handleTx(request []byte, bc *Blockchain) {
        ...
        txData := payload.Transaction
        tx := DeserializeTransaction(txData)
        mempool[hex.EncodeToString(tx.ID)] = tx

        if nodeAddress == knownNodes[0] {
            for _, node := range knownNodes {
                if node != nodeAddress && node != payload.AddFrom {
                    sendInv(node, "tx", [][]byte{tx.ID})
                }
            }
        } else {
            if len(mempool) >= 2 && len(miningAddress) > 0 {
            MineTransactions:
                var txs []*Transaction

                for id := range mempool {
                    tx := mempool[id]
                    if bc.VerifyTransaction(&tx) {
                        txs = append(txs, &tx)
                    }
                }

                if len(txs) == 0 {
                    fmt.Println("All transactions are invalid! Waiting for new ones...")
                    return
                }

                cbTx := NewCoinbaseTX(miningAddress, "")
                txs = append(txs, cbTx)

                newBlock := bc.MineBlock(txs)
                UTXOSet := UTXOSet{bc}
                UTXOSet.Reindex()

                fmt.Println("New block is mined!")

                for _, tx := range txs {
                    txID := hex.EncodeToString(tx.ID)
                    delete(mempool, txID)
                }

                for _, node := range knownNodes {
                    if node != nodeAddress {
                        sendInv(node, "block", [][]byte{newBlock.Hash})
                    }
                }

                if len(mempool) > 0 {
                    goto MineTransactions
                }
            }
        }
    }

首先要将交易放到内存池中，当然将交易放到内存池之前，必须要对其进行验证。

当一笔交易被挖出以后，就会从内存池中移除。当前节点连接到的所有其他节点，接收带有新块哈希的inv消息。在处理完消息后，可以对块进行请求。

### 代码测试命令

    // 在第一个终端(NODE_ID=3000)操作
    go build -o blockchain_go
    export NODE_ID=3000
    ./blockchain_go createwallet        //1767XRHPFFEgscVrEVN2z4tLCPmkCeXwSZ
    ./blockchain_go createblockchain -address 1767XRHPFFEgscVrEVN2z4tLCPmkCeXwSZ
    ./blockchain_go getbalance -address 1767XRHPFFEgscVrEVN2z4tLCPmkCeXwSZ
    ./blockchain_go printchain
    cp blockchain_3000.db blockchain_genesis.db

    // 打开另外一个终端(NODE_ID=3001)操作
    ./blockchain_go createwallet     //15iGjMkA5d7XR8yFU2zxUiE2vcLuHdGMmn
    ./blockchain_go createwallet     //1AUCd7jxWeH4R8TnZoGx25pziLGLWm8oJm

    // 在终端(NODE_ID=3000)操作
    ./blockchain_go send -from 1767XRHPFFEgscVrEVN2z4tLCPmkCeXwSZ -to 15iGjMkA5d7XR8yFU2zxUiE2vcLuHdGMmn -amount 10 -mine
    ./blockchain_go send -from 1767XRHPFFEgscVrEVN2z4tLCPmkCeXwSZ -to 1AUCd7jxWeH4R8TnZoGx25pziLGLWm8oJm -amount 10 -mine
    ./blockchain_go startnode  // 启动节点

    // 在终端(NODE_ID=3001)操作
    ./blockchain_go startnode  // 启动节点通信，把区块进行同步
    // 终止节点运行
    ./blockchain_go getbalance -address 1767XRHPFFEgscVrEVN2z4tLCPmkCeXwSZ
    ./blockchain_go getbalance -address 15iGjMkA5d7XR8yFU2zxUiE2vcLuHdGMmn
    ./blockchain_go getbalance -address 1AUCd7jxWeH4R8TnZoGx25pziLGLWm8oJm

### 项目代码


















