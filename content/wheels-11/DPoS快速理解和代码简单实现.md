# DPoS快速理解和代码简单实现

### 什么是DPoS

DPoS是Delegated Proof of Stake，翻译就是股份授权证明。出发点是为了解决PoW和PoS共识机制的不足。

### DPoS基本原理

PoS，股权证明机制，每个节点都可以创建区块，并按照个人的持股比例获得利息。DPoS是由被社区选举的可信账户（受托人，比如得票数前101位可以成为）来创建区块。比如EOS就是这样在社区热热闹闹不断进行拉票活动，反正用户是根据自己持有的币的数量占比获得投票权力。DPoS机制类似于股份制公司，普通股民不是董事会成员，投票选举代理人（受托人）代替他们做决策，这基本也是西方代议制民主的一个体现。

比如选出101个受托人，他们就是101个矿池，他们之间的权利是完全相等的。当然普通的拥币者可以随时通过投票更换这些代表（矿池），DPoS的去中心化不是说每个拥币者就有直接的股份权益，而是需要间接的投票权力，这样用来保证那些被推选出来的超级节点不作恶，同时也可以自己拉选票成为超级节点或者备用超级节点。

### DPoS运行机制

- 1.所有持币者选出受托人负责签署区块：选出的超级节点非常少，效率自然就高很多。
- 2.与PoW相同，DPoS的规则也是最长链胜出。每个受托人必须按照生产排程，轮流产生区块。大家必须轮到自己的时候才能出块。
![](http://chuantu.biz/t6/340/1531189758x-1376440132.png)
- 3.如果有恶意节点产生分叉，假设A，C都是诚实节点，B是恶意节点，由于B产生区块的速度慢于A，C合力产生区块的速度，最长链胜出规则，诚实节点的链会胜出。
![](http://chuantu.biz/t6/340/1531189778x-1376440132.png)
- 4.同一个节点要产生重复两个区块的速度也要慢于诚实节点合作产生区块的速度，所以最长链胜出规则会保证诚实节点的链会胜出。
- 5.如果A，B，C三个受托人的网络出现碎片化，各自为政的情况。在短期内的确可能三链并行，但一旦网络连接恢复，锻炼自然会向最长链回归。
![](http://chuantu.biz/t6/340/1531189794x-1376440132.png)

这里受托人可签署人数是奇数，两大派系势均力敌的僵局是不会长久的，最终势必会有其中一方的链更长。

### DPoS对作恶节点的惩罚

注册成为候选受托人需要支付一笔保证金，这笔保证金就是为了防止节点出现作恶的情况，一般来说，如果成为受托人，也就成为超级节点进行挖矿，超级节点需要大概两周的时间才能达到损益平衡，这就促使超级节点至少挖满两周不作恶。

如果有超级节点不按排程生产区块，就会在下一轮被投票剔除，也会被没收之前缴纳的保证金。

所以DPoS的效率高，出块速度也快得多。恶意节点在短期内是能够作恶的，这样恶意的区块只是短时间保留而已，很快超级节点之间会回归诚实节点达成的共识，制造出最长链，向没有作恶区块的最长链回归。

### DPoS的优缺点

> 优点

- 1.能耗优势，超级节点的方式，使得整个网络的能耗大大下降，网络运行成本更低。
- 2.理论上更加去中心化。为什么说是理论上，因为相比PoW的算力掌握在几个大矿池手中，而矿池是中心化的；相比PoS，小散不挖矿，大户挖矿，币也会越来越集中。而DPoS使得币更加分散化，每个拥币者都有根据币数量的权力，但实际情况是小散没啥动力投票，大户有大的话语权。
- 3.更快的确认速度。出块速度是秒级，一笔交易确认基本是分钟级别，比如一分钟就基本足够了。

> 缺点

- 1.小散投票积极性不高。其实这跟民主社会，投票积极性不高的道理是一样的，投票没有多少收益，而不投票能避免投票所花的成本（时间，精力和技能学习）。
- 2.坏节点不能被即时处理，总是要经过一个选举才能清除坏节点，这样会给网络造成安全隐患。

### 简单代码实现

说是简单代码实现，实际上复杂得一批。相比完整的区块链项目，很多区块验证，主链侧链竞争机制，区块内部的交易等都直接忽略，交易都忽略了，什么地址，钱包，非对称加密等都直接阉割。

从项目的main.go看，在四个终端用不同端口模拟四个超级节点出来。

	func StartServer(nodeID string){
	nodeAddress = fmt.Sprintf("localhost:%s",nodeID)
	log.Println("Peer start address: ",nodeAddress)
	listen,err := net.Listen(protocol,nodeAddress)
	if err != nil{
		log.Panic(err)
	}
	defer listen.Close()
	blockchain := NewBlockchain(nodeID)
	lastHeight := blockchain.GetBestHeight()
	numberDelegate := GetNumberDelegates(blockchain)
	delegate := &Delegates{nodeVersion,lastHeight,nodeAddress,numberDelegate}
	InsertDelegates(blockchain,delegate,lastHeight)
	if nodeAddress != knownNodes[0]{
		sendDelegates(blockchain,numberDelegate+1,delegate)
	}
	go Forks(blockchain)
	for {
		conn,err := listen.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleConnection(conn,blockchain)
	}
}

其主要逻辑为：

- 1.根据不同端口，tcp监听。
- 2.根据端口号新建一条区块链存放在超级节点上。
- 3.从新建区块链上获取lastHeight和delegates的数量。
- 4.根据nodeVersion，lastHeight，nodeAddress和numberDelegate构建delegate对象。
- 5.将新建的delegate对象加入到数据库。
- 6.如果不是第一个启动的节点，就需要添加新的委托人节点。具体过程下面分析。
	- sendDelegates(blockchain,numberDelegate+1,delegate)
- 7. 添加完委托人节点后，就启动协程进行轮流出块。后面分析。
	- go Forks(blockchain)
- 8.对tcp监听的结果进行处理。后面分析。

其中Delegates对象的解构如下：

	type Delegates struct {
		Version int64
		LastHeight int64
		Address string
		NumPeer int
	}
这个结构既能说明自己节点的位置，又能知道全局有多少委托人节点。还能查看到上一个出块时的区块高度。

接下来具体分析如何进行新的委托人节点添加。

	func sendDelegates(blockchain *Blockchain,numberDelegate int,delegate *Delegates){
		listDelegates := GetDelegates(blockchain)
		for _,tmpDelegate := range listDelegates{
			log.Println(tmpDelegate.Address, nodeAddress, numberDelegate, delegate.NumPeer)
			if tmpDelegate.Address != nodeAddress && numberDelegate>delegate.NumPeer{
				data := delegateSt{nodeAddress,delegate.SerializeDelegate()}
				payload := gobEncode(data)
				request := append(commandToBytes("delegates"),payload...)
				sendData(tmpDelegate.Address,request)
			}
	
			if tmpDelegate.Address != nodeAddress{
				data := delegateSt{tmpDelegate.Address,tmpDelegate.SerializeDelegate()}
				payload := gobEncode(data)
				request := append(commandToBytes("delegates"),payload...)
				sendData(delegate.Address,request)
			}
		}
	}

其主要逻辑就是获取现在已有的委托人节点列表，进行遍历，如果不是现在添加的这个节点，就要向其他节点发送消息过去。

	func sendData(address string,data []byte)  {
		conn,err := net.Dial(protocol,address)
		defer conn.Close()
		_,err = io.Copy(conn,bytes.NewReader(data))
		if err != nil {
			log.Panic(err)
		}
	}

发送消息也是非常简单的，分别是发送消息的地址说明给哪个委托人节点发送消息，然后是消息所要执行的request命令。

然后是在StartServer()中有一个协程在监听这些消息，得到消息的命令后会执行。我们看如何添加新的委托人节点。

	func handleBlock(request []byte,blockchain *Blockchain){
		var buff bytes.Buffer
		var payload block
		buff.Write(request[commandLength:])
		decoder := gob.NewDecoder(&buff)
		err := decoder.Decode(&payload)
		if err != nil {
			log.Panic(err)
		}
		blockData := payload.Block
		block := DeserializeBlock(blockData)
		log.Println("Recevied a new block!")
		blockchain.AddBlock(block)
		log.Println("Added block", block.Hash)
		listPeer := GetDelegates(blockchain)
		for _,peer := range listPeer{
			UpdateDelegate(blockchain,peer.Address,block.Height)
		}
	}

其主要逻辑就是从request中获取delegate数据，反序列化获得delegate对象，然后更新bestHeight。将新的委托人节点写入数据库。获取最新的delegates数目，如果写入成功，就再次进行广播。直到其LastHeight都同步了。

重点分析Forks()这个方法如何让委托人节点进行轮流出块的。

	// Delegates Proof of State
	func Forks(blockchain *Blockchain){
		for {
			genBlockPeriod(blockchain)
		}
	}

这个方法就是开启了一个无限循环，不断进行一轮又一轮的出块周期。那么就详细分析一个出块周期就可以了。

	func genBlockPeriod(blockchain *Blockchain){
	time.Sleep(utils.BLOCK_TIME*time.Second)
		listDelegates := GetDelegates(blockchain)
		if len(listDelegates) > 3 {
			lastBlock := blockchain.GetLastBlock()
			indexDelegate := 0
			delegate := listDelegates[indexDelegate]
			if nodeAddress == knownNodes[0]{
				quorum := 0
				noquorum := 0
				for _,delegatePeer := range listDelegates{
					if delegatePeer.LastHeight == lastBlock.Height{
						quorum += 1
					}else{
						noquorum += 1
					}
				}
				log.Println(quorum,noquorum)
				if (quorum+noquorum)>0 && float64(float64(quorum)/float64(quorum+noquorum))>float64(0.66){
					block := generateBlock(lastBlock,0,delegate.Address)
					blockchain.AddBlock(block)
					log.Println(block)
	
					for _,delegatePeer := range listDelegates{
						if delegatePeer.Address != nodeAddress{
							log.Println("sendBlock",delegatePeer.Address)
							SendBlock(delegatePeer.Address,block)
							UpdateDelegate(blockchain,delegatePeer.Address,block.Height)
						}
					}
				}
			}else{
				log.Println(delegate.Address)
			}
		}else {
			log.Println("len peer: ",len(listDelegates))
		}
	}

其主要逻辑如下：

- 1.获取所有委托人节点，判断委托人节点是否超过3个。如果不够4个，就等待。当然这个数目是可以自己设定的。
- 2.一旦委托人就绪，就获取最新的一个区块。
- 3.判断委托人节点是否是启动的第一个委托人节点，如果是的话，第一个委托人节点就负责出块，（当然这里可以改成轮流出块）。
	- if nodeAddress == knownNodes[0]{
- 4.然后根据区块高度判断其他委托人节点是否已经是最新区块，然后统计最新区块的委托人节点比例，如果超过2/3，就可以进行出块。
- 5.出完区块之后，就要进行广播。还有遍历更新委托人节点状态。
	

总结：这里没有严格按照DPoS的方式来写代码。一方面是没有在一个周期后将委托人节点进行乱序，另外一方面是没有让所有委托人节点都有出块的权力，当然也没有按照委托人的排序来进行出块进行最长链的选择机制。

代码地址：[https://github.com/jianhuaixie/blockchain-buildwheels/tree/master/content/wheels-12](https://github.com/jianhuaixie/blockchain-buildwheels/tree/master/content/wheels-12)

这个项目运行很简单：

	go build main.go
	./main.exe 3000
	
	//新开一个终端
	cp blockchain_3000.db blockchain_3001.db
	./main 3001 

	//新开一个终端
	cp blockchain_3000.db blockchain_3002.db
	./main 3002 

	//新开一个终端
	cp blockchain_3000.db blockchain_3003.db
	./main 3003