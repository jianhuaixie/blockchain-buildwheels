# BTCD区块链blockchain实现

通过BTCD的协议消息，在各个节点上进行同步transaction和block，最终在整个网络中形成一个一致的区块链。

各个节点从其他Peer上拉到transactions和block数据，就可以进行挖矿，也就是工作量证明，当然证明出结果的时候，就需要进行全网
广播，这些广播出的新块将会被其他矿工进行验证。

so，对于矿工来说，有两种方式获得区块，第一种就是通过自己挖矿产生的新区块，从其他节点拉取到的区块。

如果说区块链的整个协议底层是基于消息驱动也不为过，比如区块的广播和拉取，就是基于协议消息inv-getdata-block来实现。

当然不管是你自己作为矿工挖到的新块，还是从别的矿工那拉取到的区块，要想添加到区块链账本上，都是需要进行验证，这个在全网
按照一定规则获得一定比例的共识，最后把区块添加到区块链的过程就是共识机制。

分叉又是怎么回事呢？很简单，比如按照一定规则，这个规则发生了变化，这样就会出现分叉的现象，比如区块存放的交易数据量大小发生变化。
至于硬分叉和软分叉的差别，网上有很多学习的资料，在此不做误读。

接下来我们重点分析btcd/blockchain这个模块，其中的文件很多：

- fullblocktests：里面主要两个文件generate.go和params.go,这个包主要提供一系列区块共识验证，对于更多细节，可阅读doc.go。
- indexers：这个包实现了区块链的索引，支持两种索引，一种是根据交易hash的block索引，一种是根据address的transactions索引。
- testdata：装测试用的block数据。
- accept.go：maybeAcceptBlock()方法，区块需要添加到区块链之前，需要进行一系列验证，此方法封装了此功能集合。
- blockindex.go：每一个block在blockchain的Merkle树上都有一个hash，blockindex会实现block和这个hash的映射关系，当然block之间的前后顺序关系也会实现。
- chain.go：BlockChain结构的定义，最核心的就是区块链如何构建实现。
- chainio.go：跟数据库db进行区块以及元数据的读写方法。
- chainview.go：提供了对从tip回溯到创世块的特定块的统一的查看，以及其他比对链的大量便利的方法。
- checkpoints.go：提供Blockchain中需要持久化的方法，比如verifyCheckpoint()，findPreviousCheckpoint(),IsCheckpointCandidate(),这些ckeckpoint用来进行区块验证工作。
- compress.go：[A variable length quantity (VLQ)](https://en.wikipedia.org/wiki/Variable-length_quantity),用来对交易输出和交易输出脚本锁定进行压缩编码。
- difficulty.go：挖矿难度的实现方法，这个涉及到挖矿难度的定义。
- error.go：跟构建区块链相关的一堆错误的定义。
- log.go：logger相关的方法。
- mediantime.go：MedianTimeSource接口，提供了一种采样机制，几个时间点的采样计算一个中位时间值用来调整本地时钟偏差。
- merkle.go：当然是我最喜欢的Merkle树的算法实现啦。
- notifications.go：一系列给消费者chain event的回调方法的通知，比如NTBlockAccepted，NTBlockConnected和NTBlockDisconnected。
- process.go：对区块进行处理的逻辑。
- scriptval.go：关于脚本锁定的验证逻辑，ValidateTransactionScripts(),checkBlockScripts(),用多协程来对过去区块中所有交易进行锁定脚本的验证。
- thresholdstate.go：对于部署状态进行定义，新的共识规则需要向全网进行部署。
    - Defined：对于每一个部署的第一个状态
    - Started：开始时间到达后的部署状态
    - LockedIn：是重定向期间的部署状态，该期间位于ThresholdStarted状态期限之后，并且投票部署的块数等于或超过部署所需的投票数。
    - Active：是在部署处于ThresholdLockedIn状态的重定时期之后的所有块的部署状态。
    - Failed：在截止时间前还没有到ThresholdLockedIn状态，对于这样的部署的状态就是Failed
- timesorter.go：实现了time.Interface接口，能将时间戳slice进行排序。
- upgrade.go：对区块进行更新操作。
- utxoviewpoint.go：简单理解就是维护一个UTXO的集合，对于增删交易输入和交易输出的功能实现。
- validate.go：对区块和交易进行验证的工作。
- versionbits.go：版本号的定义，bitConditionChecker的版本号bit是uint32，每一个新挖出的区块都有一个版本号，这个版本号就是根据部署状态来的。
- weight.go：定义了一些block的weight，是在BIP0141中定义的。

### chain.go

    type BlockChain struct {
    	checkpoints         []chaincfg.Checkpoint           //链上的某些被预先设定的区块，用来进行验证
    	checkpointsByHeight map[int32]*chaincfg.Checkpoint  //checkpoints跟区块的高度之间的map映射
    	db                  database.DB                     //存储区块的数据库对象
    	chainParams         *chaincfg.Params                //与区块链相关的参数配置
    	timeSource          MedianTimeSource                //校正本地时钟的时钟源
    	sigCache            *txscript.SigCache              //缓存公钥和签名验证的结果
    	indexManager        IndexManager                    //索引链上交易或者区块的索引管理器
    	hashCache           *txscript.HashCache             //

        // 根据chainParams就决定了下面几个参数
    	minRetargetTimespan int64 // target timespan / adjustment factor        //难度调整的最小周期
    	maxRetargetTimespan int64 // target timespan * adjustment factor        //难度调整的最大周期
    	blocksPerRetarget   int32 // target timespan / target time per block    //难度调整周期内的区块数量

    	// chainLock protects concurrent access to the vast majority of the
    	// fields in this struct below this point.
    	chainLock sync.RWMutex                              //用于区块链访问的读写锁

    	// 这些字段与内存块索引相关。它们都有自己的锁，但是它们通常也受链锁保护，以便在处理块时帮助防止逻辑竞争。
    	index     *blockIndex                   //索引将整个块索引存储在内存中。块索引是一个树形结构。
    	bestChain *chainView                    //通过在块索引中使用有效的链视图来跟踪当前的活动链。

    	// 这些字段与处理孤立块有关。它们受链锁和孤儿锁的组合保护。
    	orphanLock   sync.RWMutex                       //保护孤立块相关对象的读写锁
    	orphans      map[chainhash.Hash]*orphanBlock    //是一个map，key是孤立块的hash，value是孤立块
    	prevOrphans  map[chainhash.Hash][]*orphanBlock  //是一个map，key是孤立块父区块的hash，value是父区块
    	oldestOrphan *orphanBlock                       //出于孤立状态时间最长的区块，当孤立块池超过100个孤立块，就要清掉最老的孤立块

    	// 这些字段与checkpoint处理有关。它们受链锁保护。
    	nextCheckpoint *chaincfg.Checkpoint             //下一个checkpoint点
    	checkpointNode *blockNode                       //checkpoint的节点，blockNode表示块链内的块，主要用于帮助选择最佳链作为主链。

    	// 该状态用作缓存关于当请求时返回给调用者的当前最佳链状态的信息的相当有效的方式。
        // 它按照MVCC的原理进行操作，以便每当新块成为最佳块时，状态指针将被替换为新的结构，而旧状态保持不变。
        // 这样，多个呼叫者可以指向不同的最佳链状态。对于大多数呼叫者来说，这是可以接受的，因为状态只在特定的时间点被查询。
    	// 另外，一些字段存储在数据库中，因此可以在加载时快速重建链状态。
    	stateLock     sync.RWMutex                      //保护stateSnapshot的读写锁
    	stateSnapshot *BestState                        //主链相关信息的快照

    	// 当如何规则进行更改的时候，下面的缓存是用来高效跟踪当前部署状态
    	// 因为这些被保存在在数据库里，所以能非常快速地进行在线重建
    	// warningCaches缓存每个**可能**部署中块的当前部署阈值状态。这用于检测新的无法识别的规则更改何时被投票和/或已被激活，例如当正在使用较旧版本的软件时
    	//
    	// deploymentCaches caches the current deployment threshold state for
    	// blocks in each of the actively defined deployments.
    	warningCaches    []thresholdStateCache  //缓存所有thresholdState的数组，用来提醒更新btcd的版本，机制是大量新块使用新版本，且对应的共识规则在新块部署
    	deploymentCaches []thresholdStateCache  //缓存部署状态thresholdState的数组

    	unknownRulesWarned    bool  //是否已经警告过未知共识规则已部署或即将被部署的标识
    	unknownVersionsWarned bool  //是否已经警告过未知新版本的区块已经收到

    	// 这些通知的字段用来存储了一系列的回调，这些回调方法将被用在区块events上
    	notificationsLock sync.RWMutex              //用于通知的读写锁
    	notifications     []NotificationCallback    //这些回调的数组
    }


ProcessBlock是处理将新块插入块链中的主要工具。它包括诸如拒绝重复块，确保块遵循所有规则，孤立处理以及插入块链以及最佳链选择和重组等功能。

    func (b *BlockChain) ProcessBlock(block *btcutil.Block, flags BehaviorFlags) (bool, bool, error) {
    	b.chainLock.Lock()
    	defer b.chainLock.Unlock()

    	fastAdd := flags&BFFastAdd == BFFastAdd

    	blockHash := block.Hash()
    	log.Tracef("Processing block %v", blockHash)

    	// The block must not already exist in the main chain or side chains.
    	exists, err := b.blockExists(blockHash)
    	if err != nil {
    		return false, false, err
    	}
    	if exists {
    		str := fmt.Sprintf("already have block %v", blockHash)
    		return false, false, ruleError(ErrDuplicateBlock, str)
    	}

    	// The block must not already exist as an orphan.
    	if _, exists := b.orphans[*blockHash]; exists {
    		str := fmt.Sprintf("already have block (orphan) %v", blockHash)
    		return false, false, ruleError(ErrDuplicateBlock, str)
    	}

    	// Perform preliminary sanity checks on the block and its transactions.
    	err = checkBlockSanity(block, b.chainParams.PowLimit, b.timeSource, flags)
    	if err != nil {
    		return false, false, err
    	}

    	// 找到之前的检查点，并根据检查点执行额外的检查。
    	// 这些额外的检查提供了一些很好的属性，比如：可以阻止在最后的检查点上出现老的侧链区块。
    	// 可以拒绝那些用来吞噬内存的伪造区块被容易挖到
    	// 在之前检查点被遇见前，可以确保逾期的工作量证明
    	blockHeader := &block.MsgBlock().Header
    	checkpointNode, err := b.findPreviousCheckpoint()
    	if err != nil {
    		return false, false, err
    	}
    	if checkpointNode != nil {
    		// Ensure the block timestamp is after the checkpoint timestamp.
    		checkpointTime := time.Unix(checkpointNode.timestamp, 0)
    		if blockHeader.Timestamp.Before(checkpointTime) {
    			str := fmt.Sprintf("block %v has timestamp %v before "+
    				"last checkpoint timestamp %v", blockHash,
    				blockHeader.Timestamp, checkpointTime)
    			return false, false, ruleError(ErrCheckpointTimeTooOld, str)
    		}
    		if !fastAdd {
    			// 即便之前的检查已经能够确保工作量超过了声明总量，但是在区块头中的声明总量是可以被伪造的。
    			// 此检查可确保工作量证明至少基于自上次检查以来一段时间内通过重定向规则允许的最大调整的预期最小值。
    			duration := blockHeader.Timestamp.Sub(checkpointTime)
    			requiredTarget := CompactToBig(b.calcEasiestDifficulty(
    				checkpointNode.bits, duration))
    			currentTarget := CompactToBig(blockHeader.Bits)
    			if currentTarget.Cmp(requiredTarget) > 0 {
    				str := fmt.Sprintf("block target difficulty of %064x "+
    					"is too low when compared to the previous "+
    					"checkpoint", currentTarget)
    				return false, false, ruleError(ErrDifficultyTooLow, str)
    			}
    		}
    	}

    	// Handle orphan blocks.
    	prevHash := &blockHeader.PrevBlock
    	prevHashExists, err := b.blockExists(prevHash)
    	if err != nil {
    		return false, false, err
    	}
    	if !prevHashExists {
    		log.Infof("Adding orphan block %v with parent %v", blockHash, prevHash)
    		b.addOrphanBlock(block)

    		return false, true, nil
    	}

    	// The block has passed all context independent checks and appears sane
    	// enough to potentially accept it into the block chain.
    	isMainChain, err := b.maybeAcceptBlock(block, flags)
    	if err != nil {
    		return false, false, err
    	}

    	// Accept any orphan blocks that depend on this block (they are
    	// no longer orphans) and repeat for those accepted blocks until
    	// there are no more.
    	err = b.processOrphans(blockHash, flags)
    	if err != nil {
    		return false, false, err
    	}

    	log.Debugf("Accepted block %v", blockHash)

    	return isMainChain, false, nil
    }

ProcessBlock()输入参数的block是*btcutil.Block类型，其定义如下：

    type Block struct {
    	msgBlock                 *wire.MsgBlock  // Underlying MsgBlock
    	serializedBlock          []byte          // Serialized bytes for the block
    	serializedBlockNoWitness []byte          // Serialized bytes for block w/o witness data
    	blockHash                *chainhash.Hash // Cached block hash
    	blockHeight              int32           // Height in the main block chain
    	transactions             []*Tx           // Transactions
    	txnsGenerated            bool            // ALL wrapped transactions generated
    }

ProcessBlock()返回 return isMainChain, false, nil ，isMainChain表示区块是否已经加入了主链，第二个bool值表示是否是孤立块

其处理逻辑如下：

- 1.检查区块是否已经在链上。
    - exists, err := b.blockExists(blockHash)
- 2.检查区块是否在孤立块池中，孤立块池默认能放100个孤立块
    - if _, exists := b.orphans[*blockHash]; exists {
- 3.checkBlockSanity()方法对区块进行检查，可以点击进去查看，检查内容包括区块头checkBlockHeaderSanity()，区块中交易数量检查，区块大小检查，checkpoint检查，交易集合检查CheckTransactionSanity(),merkle根检查等
    - err = checkBlockSanity(block, b.chainParams.PowLimit, b.timeSource, flags)
- 4.基于最近的checkpoint与区块之间的时间差，计算逾期的最小工作量，通过比较预期最小工作量和区块的工作量，防止checkpoint机制中伪造工作量证明
    - currentTarget := CompactToBig(blockHeader.Bits)
    -  if currentTarget.Cmp(requiredTarget) > 0 {  //如果区块的工作量低于预期的最小工作量则被拒绝
- 5.通过checkpoint后，检查区块的父区块是否在链上，如果不在，当然这个块就是孤立块，就直接放到孤立块池。
    -  b.addOrphanBlock(block)
- 6.maybeAcceptBlock()方法对区块进行上下文检查，根据块是否为孤立块来扩展主链还是侧链，并需要对区块中的交易进行验证。
    - 扩展主链，验证通过后，将区块加入主链，
    - 扩展侧链，比较侧链和主链的工作量，判断是否需要主侧角色互换
- 7.区块加入区块链后，processOrphans(blockHash, flags)方法检测孤立块池中是否有孤立块的父区块刚接入主链，这些孤立块将不再孤立，有块“脱孤”，就重复此过程。

通过上面的分析，我们画一个ProcessBlock()的简单流程图：


接下来，看看在ProcessBlock()方法中要用到的验证的方法的详细过程，比如checkBlockSanity()，maybeAcceptBlock()，processOrphans()等。

    func checkBlockSanity(block *btcutil.Block, powLimit *big.Int, timeSource MedianTimeSource, flags BehaviorFlags) error {
    	msgBlock := block.MsgBlock()
    	header := &msgBlock.Header
    	err := checkBlockHeaderSanity(header, powLimit, timeSource, flags)
    	if err != nil {
    		return err
    	}

    	// A block must have at least one transaction.
    	numTx := len(msgBlock.Transactions)
    	if numTx == 0 {
    		return ruleError(ErrNoTransactions, "block does not contain "+
    			"any transactions")
    	}

    	// 一个块不能有比最大块有效载荷更多的交易，否则它肯定超过了权重限制。
    	if numTx > MaxBlockBaseSize {
    		str := fmt.Sprintf("block contains too many transactions - "+
    			"got %d, max %d", numTx, MaxBlockBaseSize)
    		return ruleError(ErrBlockTooBig, str)
    	}

    	//在序列化时，块不得超过允许的最大块有效载荷。
    	serializedSize := msgBlock.SerializeSizeStripped()
    	if serializedSize > MaxBlockBaseSize {
    		str := fmt.Sprintf("serialized block is too big - got %d, "+
    			"max %d", serializedSize, MaxBlockBaseSize)
    		return ruleError(ErrBlockTooBig, str)
    	}

    	// The first transaction in a block must be a coinbase.
    	transactions := block.Transactions()
    	if !IsCoinBase(transactions[0]) {
    		return ruleError(ErrFirstTxNotCoinbase, "first transaction in "+
    			"block is not a coinbase")
    	}

    	// A block must not have more than one coinbase.
    	for i, tx := range transactions[1:] {
    		if IsCoinBase(tx) {
    			str := fmt.Sprintf("block contains second coinbase at "+
    				"index %d", i+1)
    			return ruleError(ErrMultipleCoinbases, str)
    		}
    	}

    	// Do some preliminary checks on each transaction to ensure they are sane before continuing.
    	for _, tx := range transactions {
    		err := CheckTransactionSanity(tx)
    		if err != nil {
    			return err
    		}
    	}

    	// 构建merkle树并确保计算的merkle根与块头中的条目匹配。这也可以缓存块中的所有事务哈希以加速未来的哈希检查。
    	merkles := BuildMerkleTreeStore(block.Transactions(), false)
    	calculatedMerkleRoot := merkles[len(merkles)-1]
    	if !header.MerkleRoot.IsEqual(calculatedMerkleRoot) {
    		str := fmt.Sprintf("block merkle root is invalid - block "+
    			"header indicates %v, but calculated value is %v",
    			header.MerkleRoot, calculatedMerkleRoot)
    		return ruleError(ErrBadMerkleRoot, str)
    	}

    	// 检查重复的交易。这个检查会很快，因为事务哈希已经被缓存了，因为在上面构建了merkle树。
    	existingTxHashes := make(map[chainhash.Hash]struct{})
    	for _, tx := range transactions {
    		hash := tx.Hash()
    		if _, exists := existingTxHashes[*hash]; exists {
    			str := fmt.Sprintf("block contains duplicate "+
    				"transaction %v", hash)
    			return ruleError(ErrDuplicateTx, str)
    		}
    		existingTxHashes[*hash] = struct{}{}
    	}

    	// 签名操作的数量必须少于每个块允许的最大数量。
    	totalSigOps := 0
    	for _, tx := range transactions {
    		// 我们可能会溢出累加器，检查溢出。
    		lastSigOps := totalSigOps
    		totalSigOps += (CountSigOps(tx) * WitnessScaleFactor)
    		if totalSigOps < lastSigOps || totalSigOps > MaxBlockSigOpsCost {
    			str := fmt.Sprintf("block contains too many signature "+
    				"operations - got %v, max %v", totalSigOps,
    				MaxBlockSigOpsCost)
    			return ruleError(ErrTooManySigOps, str)
    		}
    	}
    	return nil
    }

checkBlockSanity的验证有10个验证，分别是：

- 1.checkBlockHeaderSanity()区块头进行验证，下面会分析。
- 2.对区块中的交易进行检查，至少需要有一笔交易。
- 3.对区块中的交易总数量检查，直接就是对block的最大长度进行限制，不能超过1000000字节。
- 4.将区块序列化后，检查其大小，也是不能超过1000000字节，大约1M。
- 5.对区块中coinbase进行检查，只能有一个，且是区块中第一个交易。
- 6.对区块中coinbase进行检查，如果有多个coinbase交易，就报错。
- 7.对区块中每个交易进行检查，CheckTransactionSanity()。
- 8.BuildMerkleTreeStore()方法用交易集合构建一颗merkle树，然后用Merkle树根与区块头中的Merkle树根比对，防止交易集合中有交易被篡改。
- 9.对区块中交易集合中是否有重复的交易，并不是对UTXO进行双花检查。
- 10.对区块中所有交易的解锁脚本和锁定脚本中的操作符的总数进行检查，如果超过了限制，这个区块可能有恶意脚本来消耗节点计算资源。

    func checkBlockHeaderSanity(header *wire.BlockHeader, powLimit *big.Int, timeSource MedianTimeSource, flags BehaviorFlags) error {
    	// 确保在区块头中的bits的工作量证明是在最大和最小范围内，并且区块的hash要比在bits中描述的target要小
    	err := checkProofOfWork(header, powLimit, flags)
    	if err != nil {
    		return err
    	}

    	// 块时间戳的精度不能超过1秒。因为Go time.Time的时间戳支持纳秒精度，
    	// 而共识规则只需要秒级别精度，因此使用Go的标准时间戳是更好的选择。
    	if !header.Timestamp.Equal(time.Unix(header.Timestamp.Unix(), 0)) {
    		str := fmt.Sprintf("block timestamp of %v has a higher "+
    			"precision than one second", header.Timestamp)
    		return ruleError(ErrInvalidTime, str)
    	}

    	// Ensure the block time is not too far in the future.
    	maxTimestamp := timeSource.AdjustedTime().Add(time.Second *
    		MaxTimeOffsetSeconds)
    	if header.Timestamp.After(maxTimestamp) {
    		str := fmt.Sprintf("block timestamp of %v is too far in the "+
    			"future", header.Timestamp)
    		return ruleError(ErrTimeTooNew, str)
    	}
    	return nil
    }

可以看到，checkBlockHeaderSanity()方法是对区块头中的难度值和时间戳进行检查，有三大检查，如下：

- 1.checkProofOfWork()方法的调用是对工作量进行检查。
- 2.检查时间戳的精度是否为1秒。
- 3.检查区块头中时间戳是否超过本地当前时间2小时。这个本地时间当然是经过Peer节点同步校正过的时间。

checkProofOfWork()对工作量证明的检查方法：

    func checkProofOfWork(header *wire.BlockHeader, powLimit *big.Int, flags BehaviorFlags) error {
    	// The target difficulty must be larger than zero.
    	target := CompactToBig(header.Bits)
    	if target.Sign() <= 0 {
    		str := fmt.Sprintf("block target difficulty of %064x is too low",
    			target)
    		return ruleError(ErrUnexpectedDifficulty, str)
    	}

    	// The target difficulty must be less than the maximum allowed.
    	if target.Cmp(powLimit) > 0 {
    		str := fmt.Sprintf("block target difficulty of %064x is "+
    			"higher than max of %064x", target, powLimit)
    		return ruleError(ErrUnexpectedDifficulty, str)
    	}

    	// 块散列必须小于声明的目标，除非设置了避免工作量证明检查的标志。
    	if flags&BFNoPoWCheck != BFNoPoWCheck {
    		// The block hash must be less than the claimed target.
    		hash := header.BlockHash()
    		hashNum := HashToBig(&hash)
    		if hashNum.Cmp(target) > 0 {
    			str := fmt.Sprintf("block hash of %064x is higher than "+
    				"expected max of %064x", hashNum, target)
    			return ruleError(ErrHighHash, str)
    		}
    	}
    	return nil
    }

checkProofOfWork()方法主要是三部分的检查：

- 1.CompactToBig()方法将区块头中的Bits（目标难度）转成big number，首先这个大整数值要大于零。
- 2.检查大整数值的范围合法性。
- 3.调用HashToBig()将区块头中Hash转成大整数值，并与目标难度值进行比较。如果区块头Hash值大于目标难度值，返回该区块不满足工作量证明。


CompactToBig()方法是如何将一个32位的bit序列转成难度值：

    func CompactToBig(compact uint32) *big.Int {
    	// Extract the mantissa, sign bit, and exponent.
    	mantissa := compact & 0x007fffff
    	isNegative := compact&0x00800000 != 0
    	exponent := uint(compact >> 24)

    	// Since the base for the exponent is 256, the exponent can be treated
    	// as the number of bytes to represent the full 256-bit number.  So,
    	// treat the exponent as the number of bytes and shift the mantissa
    	// right or left accordingly.  This is equivalent to:
    	// N = mantissa * 256^(exponent-3)
    	var bn *big.Int
    	if exponent <= 3 {
    		mantissa >>= 8 * (3 - exponent)
    		bn = big.NewInt(int64(mantissa))
    	} else {
    		bn = big.NewInt(int64(mantissa))
    		bn.Lsh(bn, 8*(exponent-3))
    	}

    	// Make it negative if the sign bit is set.
    	if isNegative {
    		bn = bn.Neg(bn)
    	}

    	return bn
    }

区块头中目标难度的编码方式如下：

    //	-------------------------------------------------
    //	|   Exponent     |    Sign    |    Mantissa     |
    //	-------------------------------------------------
    //	| 8 bits [31-24] | 1 bit [23] | 23 bits [22-00] |
    //	-------------------------------------------------

    // The formula to calculate N is:
    // 	N = (-1^sign) * mantissa * 256^(exponent-3)

上面，sign指符号位对应的值（0或1），mantissa表示尾数，exponent表示指数。

CompactToBig()根据以上规则编码难度值的，相反，BigToCompact()按照上规则将难度值转换成32位bit序列。

在进行完区块头检查（时间戳和工作量证明相关的难度值和头部hash）后，接下来就要看看如何对交易进行检查。

    func CheckTransactionSanity(tx *btcutil.Tx) error {
    	// A transaction must have at least one input.
    	msgTx := tx.MsgTx()
    	if len(msgTx.TxIn) == 0 {
    		return ruleError(ErrNoTxInputs, "transaction has no inputs")
    	}

    	// A transaction must have at least one output.
    	if len(msgTx.TxOut) == 0 {
    		return ruleError(ErrNoTxOutputs, "transaction has no outputs")
    	}

    	// 序列化时，交易不得超过允许的最大块有效负载。
    	serializedTxSize := tx.MsgTx().SerializeSizeStripped()
    	if serializedTxSize > MaxBlockBaseSize {
    		str := fmt.Sprintf("serialized transaction is too big - got "+
    			"%d, max %d", serializedTxSize, MaxBlockBaseSize)
    		return ruleError(ErrTxTooBig, str)
    	}

    	// 确保交易金额在范围内
    	// 每个交易输出不得为负数，也不能超过每笔交易允许的最大数量
    	// 所有交易输出的综合必须遵守相同的限制
    	// 交易中的所有金额均以satoshi的单位值计算
    	var totalSatoshi int64
    	for _, txOut := range msgTx.TxOut {
    		satoshi := txOut.Value
    		if satoshi < 0 {
    			str := fmt.Sprintf("transaction output has negative "+
    				"value of %v", satoshi)
    			return ruleError(ErrBadTxOutValue, str)
    		}
    		if satoshi > btcutil.MaxSatoshi {
    			str := fmt.Sprintf("transaction output value of %v is "+
    				"higher than max allowed value of %v", satoshi,
    				btcutil.MaxSatoshi)
    			return ruleError(ErrBadTxOutValue, str)
    		}

    		// 二进制补码int64溢出保证任何溢出都能被检测到和被报告。
    	    // 对于比特币来说，这是不可能的情况，但是如果增加总量供应上线的话，这是有可能的。
    		totalSatoshi += satoshi
    		if totalSatoshi < 0 {
    			str := fmt.Sprintf("total value of all transaction "+
    				"outputs exceeds max allowed value of %v",
    				btcutil.MaxSatoshi)
    			return ruleError(ErrBadTxOutValue, str)
    		}
    		if totalSatoshi > btcutil.MaxSatoshi {
    			str := fmt.Sprintf("total value of all transaction "+
    				"outputs is %v which is higher than max "+
    				"allowed value of %v", totalSatoshi,
    				btcutil.MaxSatoshi)
    			return ruleError(ErrBadTxOutValue, str)
    		}
    	}

    	// 检查重复的交易输入
    	existingTxOut := make(map[wire.OutPoint]struct{})
    	for _, txIn := range msgTx.TxIn {
    		if _, exists := existingTxOut[txIn.PreviousOutPoint]; exists {
    			return ruleError(ErrDuplicateTxInputs, "transaction "+
    				"contains duplicate inputs")
    		}
    		existingTxOut[txIn.PreviousOutPoint] = struct{}{}
    	}

    	// Coinbase脚本长度必须在最小和最大长度之间。
    	if IsCoinBase(tx) {
    		slen := len(msgTx.TxIn[0].SignatureScript)
    		if slen < MinCoinbaseScriptLen || slen > MaxCoinbaseScriptLen {
    			str := fmt.Sprintf("coinbase transaction script length "+
    				"of %d is out of range (min: %d, max: %d)",
    				slen, MinCoinbaseScriptLen, MaxCoinbaseScriptLen)
    			return ruleError(ErrBadCoinbaseScriptLen, str)
    		}
    	} else {
    		// 此交易输入引用的以前交易输出不能为空。
    		for _, txIn := range msgTx.TxIn {
    			if isNullOutpoint(&txIn.PreviousOutPoint) {
    				return ruleError(ErrBadTxInput, "transaction "+
    					"input refers to previous output that "+
    					"is null")
    			}
    		}
    	}
    	return nil
    }

对交易进行检查的方法CheckTransactionSanity()的输入tx *btcutil.Tx是指向btcutil.Tx的指针，而btcutil.Tx是对wire.MsgTx的封装：

    type Tx struct {
    	msgTx         *wire.MsgTx     // Underlying MsgTx
    	txHash        *chainhash.Hash // Cached transaction hash
    	txHashWitness *chainhash.Hash // Cached transaction witness hash
    	txHasWitness  *bool           // If the transaction has witness data
    	txIndex       int             // Position within a block or TxIndexUnknown
    }

Tx定义了一个比特币的交易，可以更简单，更高效处理原始交易，还记录了第一次访问交易时的hash，这样方便后续访问不必重复相对昂贵的hashing操作。

CheckTransactionSanity()的检查有7大部分：

- 1.一个交易至少含有一个交易输入，当然有一个例外情况，那就是coinbase交易，其交易输入指向一二无效的utxo
- 2.一个交易至少含有一个交易输出
- 3.交易的size不能超过block的最大size，也就是1000000字节
- 4.交易的每个输出的比特币数量和总的交易输出数量必须大于零，且要小于2100万的发行总量
- 5.检查交易中是否有重复的交易输入
- 6.检查交易输入引用的交易是否正常，如果是coinbase交易，因为没有有效的输入，所以其解锁脚本可以是创始人写入的任意字符串，但编码后的长度要符合要求
- 7.如果不是coinbase交易，其交易输入必须指向一个有效的交易的输出utxo，其交易输入hash必须不是零且输出的索引号不是0xFFFFFFFF（表示coinbase中的交易输出）。

在对区块头和每个交易都进行完整性检查后，checkBlockSanity()还要对区块头中的Merkle根进行检查。
其实Merkle树是非常简单的概念，不懂的可以在网上找到资料稍微学习一下就懂了。
首先是利用区块中的所有交易构建一颗Merkle树，方法是BuildMerkleTreeStore()，当然在计算每个交易的Hash值的时候，肯定是先找util.Tx的txHash缓存。

当然在除了验证区块时需要构建Merkle树，在挖矿前打包区块时也要计算区块头的Merkle树根。

    func BuildMerkleTreeStore(transactions []*btcutil.Tx, witness bool) []*chainhash.Hash {
    	// 计算需要多少个条目才能将二进制Merkle树保存为线性数组，并创建一个具有该大小的数组。
    	nextPoT := nextPowerOfTwo(len(transactions))
    	arraySize := nextPoT*2 - 1
    	merkles := make([]*chainhash.Hash, arraySize)

    	// 创建一个基本的交易hashes然后将他们填充数组
    	for i, tx := range transactions {
    		// 如果我们计算的是witness merkle root，而不是常规txid，那我们使用修改的wtxid，其中包含交易中的witness数据。
    		// 如果是coinbase，其wtxid全是零。
    		switch {
    		case witness && i == 0:
    			var zeroHash chainhash.Hash
    			merkles[i] = &zeroHash
    		case witness:
    			wSha := tx.MsgTx().WitnessHash()
    			merkles[i] = &wSha
    		default:
    			merkles[i] = tx.Hash()
    		}

    	}

    	// Start the array offset after the last transaction and adjusted to the
    	// next power of two.
    	// 在上次交易后开始数组便宜并调整为下一个二次幂
    	offset := nextPoT
    	for i := 0; i < arraySize-1; i += 2 {
    		switch {
    		// 当没有左子节点时，其父节点也是nil
    		case merkles[i] == nil:
    			merkles[offset] = nil

    		// 当没有右子节点时，其父节点是根据左子节点和自己本身来生成（hashing）父节点
    		case merkles[i+1] == nil:
    			newHash := HashMerkleBranches(merkles[i], merkles[i])
    			merkles[offset] = newHash

    		// 通常情况下，将父节点设置为左子节点和右子节点结合后进行两次sha256
    		default:
    			newHash := HashMerkleBranches(merkles[i], merkles[i+1])
    			merkles[offset] = newHash
    		}
    		offset++
    	}

    	return merkles
    }

BuildMerkleTreeStore()返回的是一个有长度的slice，简单理解成一棵树，实际上存在一个线性的数组结构slice中。
但是便于索引每个节点，将Merkle树扩展为一颗完全平衡二叉树，简单理解二叉树就是叶子节点的个数会被扩充为大于或等于当前叶子节点数的最小的2的幂。
扩充的叶子节点为nil，两两组合后的父节点仍然是nil，并不会影响最后的Merkle根节点的值。

上面BuildMekleTreeStore()的逻辑：

- 1.根据交易个数，即叶子节点个数，计算slice的长度。 arraySize = [2^(logX+1)]*2-1  X是交易个数
- 2.计算出完全平衡二叉树的节点个数后，就生成slice，将merkle树各个节点先赋值为nil。
- 3.计算交易的Hash值，并按顺序填入merkles slice中，扩充的节点当然就不用管，还是为nil。
- 4.填充完后，重新遍历整个merkles slice，计算父节点的值，offset为父节点的索引值。
    - 遍历过程中，i总是偶数，所以总是左子节点，如果i节点为nil，则它和右边兄弟节点都是扩充节点，其父节点也相应为nil。
    - 如果i节点不为nil，但i+1节点为空，右子节点为空，那就需要根据规则将父节点的hash值计算出来。
    - 如果左右节点都不是nil，则直接用两者的值经过两次sha256计算出父节点的值。
- 6.返回merkles的slice

到此，区块结构检查的过程就分析完了，但是并不包括对区块上下文，也就是每个区块在区块链上的顺序的检查还没有分析。上面整个检查的流程图如下：

区块验证和Checkpoint工作量验证通过后，就会调用maybeAcceptBlock()结合区块链上下文对区块进行进一步的检查，通过验证后的区块才能
写入区块链。

maybeAcceptBlock()将会进行如下的验证：

- 基于链上预期的难度值进一步检查区块头中的难度值
- 交易中的输入是否可花费（spendable）
- 是否有双重支付
- 是否有重复交易
- 是否需要延长主链
- 是否需要延长侧链
- 是否需要将侧链变成主链

在进行完这些检查后，就可以将区块加入区块链：
- 紧接着就是将区块交易中花费的UTXO(s)变成spentTxOut
- 新生成的UTXO(s)添加到UTXO集合中
- 更新交易池mempool
- 通知其他矿工不要再挖这个块，进入下一个区块的挖掘工作


    func (b *BlockChain) maybeAcceptBlock(block *btcutil.Block, flags BehaviorFlags) (bool, error) {
    	// 这个块的高度要比前一个引用的块的高度高一个
    	prevHash := &block.MsgBlock().Header.PrevBlock
    	prevNode := b.index.LookupNode(prevHash)
    	if prevNode == nil {
    		str := fmt.Sprintf("previous block %s is unknown", prevHash)
    		return false, ruleError(ErrPreviousBlockUnknown, str)
    	} else if b.index.NodeStatus(prevNode).KnownInvalid() {
    		str := fmt.Sprintf("previous block %s is known to be invalid", prevHash)
    		return false, ruleError(ErrInvalidAncestorBlock, str)
    	}

    	blockHeight := prevNode.height + 1
    	block.SetHeight(blockHeight)

    	// 在区块链上，这个块的位置就决定了这个块需要通过所有的验证规则
    	err := b.checkBlockContext(block, prevNode, flags)
    	if err != nil {
    		return false, err
    	}

    	// 如果在数据库中，这个区块还不存在，就写入该块到数据库。
    	// 即便区块通过了所有工作量验证，但该块最终无法连接的情况，这意味着攻击者用一堆无法连接的区块来填充磁盘的成本过高。
    	// 所以将区块下载和昂贵的连接逻辑中解耦出来是有必要的。
    	// 这种解耦将具有一些很好的属性，例如使块不会成为主链的一部分，或者将无法连接的块用于进一步分析。
    	err = b.db.Update(func(dbTx database.Tx) error {
    		return dbStoreBlock(dbTx, block)
    	})
    	if err != nil {
    		return false, err
    	}

    	// 为该区块创建一个新的块节点并将其添加到节点索引。即使该区块最终连接到主链，当然其是从侧链开始的。
    	blockHeader := &block.MsgBlock().Header
    	newNode := newBlockNode(blockHeader, prevNode)
    	newNode.status = statusDataStored

    	b.index.AddNode(newNode)
    	err = b.index.flushToDB()
    	if err != nil {
    		return false, err
    	}

    	// 根据工作量证明来选择正确的链，然后将区块连接到链上。这也处理交易脚本的验证。
    	isMainChain, err := b.connectBestChain(newNode, block, flags)
    	if err != nil {
    		return false, err
    	}

    	// 通知调用者新块被接受到块链中。呼叫者通常希望通过将库存转发给其他Peers来作出反应。
    	b.chainLock.Unlock()
    	b.sendNotification(NTBlockAccepted, block)
    	b.chainLock.Lock()
    	return isMainChain, nil
    }

maybeAcceptBlock()的逻辑：

- 1.根据区块的prevHash来查找父区块。Blockchain结构中的index存储的是blockIndex结构，用于索引实例化后内存中的各区块。
    - prevNode := b.index.LookupNode(prevHash)
    - node := bi.index[*hash]
- 2.找到父区块后，将当前区块的高度值设为父区块高度+1。
    - block.SetHeight(blockHeight)
    - b.blockHeight = height
- 3.调用checkBlockContext()方法进行区块上下文的检查。下面会深入分析。
    - err := b.checkBlockContext(block, prevNode, flags)
- 4.区块通过验证后，将其写入数据库（区块文件）
    - return dbStoreBlock(dbTx, block)
- 5.为当前区块创建实例化对象blockNode，并将区块添加到内存索引中。
    - newNode := newBlockNode(blockHeader, prevNode)
    - b.index.AddNode(newNode)
- 6.在区块的上下文检查通过后，将区块写入区块链，方法是connectBestChain()。
- 7.当区块成功写入主链或者侧链后，向blockManager通知NTBlockAccepted事件，blockManager会向所有Peer节点广播发送inv或header消息，将新区块广播给Peers。

在maybeAcceptBlock()中，对区块的处理会有一个blockNode的结构体：

    // blockNode用来表示区块链内的块，主要用来帮助节点选择最佳链作为主链。主链存储到区块数据库中。
    type blockNode struct {
    	// 注意：如果不考虑在64位平台上如何影响对齐，则不应更改对此结构中定义顺序的添加，删除或修改。
    	// 目前的订单是专门制作的，以减少填充。内存中将会有数十万个这样的内容，所以需要额外的填充字节。
    	// parent is the parent block for this node.
    	parent *blockNode
    	// hash is the double sha 256 of the block.
    	hash chainhash.Hash
    	// workSum is the total amount of work in the chain up to and including this node.
    	workSum *big.Int
    	// height is the position in the block chain.
    	height int32
    	// 来自区块头的一些字段有助于选择最佳的链，并从内存中重构区块头。
    	// 这些必须被视为不可变的，并有意为了避免在64位平台上进行填充。
    	version    int32
    	bits       uint32
    	nonce      uint32
    	timestamp  int64
    	merkleRoot chainhash.Hash

    	// 状态是表示块的验证状态的bitfield。不同于其他字段，状态字段可能会被写入，
    	// 因此只能在节点已经添加到全局索引后，使用blockIndex上的并发安全NodeStatus方法访问。
    	status blockStatus
    }

blockNode可以简单看成是区块在内存中的实例化结构体。

找到父区块在内存中的blockNode对象后，调用checkBlockContext()来检查当前区块和父区块的上下承接关系：

    func (b *BlockChain) checkBlockContext(block *btcutil.Block, prevNode *blockNode, flags BehaviorFlags) error {
    	// Perform all block header related validation checks.
    	header := &block.MsgBlock().Header
    	err := b.checkBlockHeaderContext(header, prevNode, flags)
    	if err != nil {
    		return err
    	}

    	fastAdd := flags&BFFastAdd == BFFastAdd
    	if !fastAdd {
    		// 获取部署的CSV软叉的最新状态，以便根据当前的BIP 9版本位状态适当地保护新的验证行为。
    		csvState, err := b.deploymentState(prevNode, chaincfg.DeploymentCSV)
    		if err != nil {
    			return err
    		}

    		// 一旦CSV软分叉处于完全激活状态，我们将切换到使用过去分块时间戳的当前中值时间，用于所有基于锁定时间的检查。
    		blockTime := header.Timestamp
    		if csvState == ThresholdActive {
    			blockTime = prevNode.CalcPastMedianTime()
    		}

    		// 这个块的高度比前面引用到的块高一个。
    		blockHeight := prevNode.height + 1

    		// Ensure all transactions in the block are finalized.
    		for _, tx := range block.Transactions() {
    			if !IsFinalizedTransaction(tx, blockHeight,
    				blockTime) {

    				str := fmt.Sprintf("block contains unfinalized "+
    					"transaction %v", tx.Hash())
    				return ruleError(ErrUnfinalizedTx, str)
    			}
    		}

    		// 对于版本为serializedHeightVersion（2）或更新版本的块，一旦大部分网络已升级,确保coinbase以序列化块高度开始。这是BIP0034的一部分。
    		if ShouldHaveSerializedBlockHeight(header) &&
    			blockHeight >= b.chainParams.BIP0034Height {

    			coinbaseTx := block.Transactions()[0]
    			err := checkSerializedHeight(coinbaseTx, blockHeight)
    			if err != nil {
    				return err
    			}
    		}

    		// 查询Segwit软叉部署的版本位状态。如果segwit处于活动状态，我们将切换到执行所有新规则。
    		segwitState, err := b.deploymentState(prevNode,
    			chaincfg.DeploymentSegwit)
    		if err != nil {
    			return err
    		}

    		// 如果segwit处于活动状态，那么我们需要全面验证新证人对遵守规则的承诺
    		if segwitState == ThresholdActive {
    			// 验证块内的证人承诺（如果有）。这涉及断言如果coinbase包含特殊承诺输出，那么这个merkle根就匹配块内交易的所有wtxid的计算merkle根。
    			// 此外，还有其他各种针对coinbase证人堆栈的检查。
    			if err := ValidateWitnessCommitment(block); err != nil {
    				return err
    			}

    			// 一旦证人承诺，证人随机数和签字成本得到验证，我们终于可以断言该区块的权重不会超过当前的共识参数。
    			blockWeight := GetBlockWeight(block)
    			if blockWeight > MaxBlockWeight {
    				str := fmt.Sprintf("block's weight metric is "+
    					"too high - got %v, max %v",
    					blockWeight, MaxBlockWeight)
    				return ruleError(ErrBlockWeightTooHigh, str)
    			}
    		}
    	}
    	return nil
    }

checkBlockContext()的主要逻辑：

- 1.checkBlockHeaderContext()对区块头进行上下文检查。
- 2.deploymentState()方法计算父区块的子区块的CSV部署的状态。
    - 如果是Active状态，就通过CalcPastMedianTime()方法计算blockTime
    - 采用MTP(Median Time Past)，而不是区块头中懂得tiemstamp来检查比较交易的LockTime，防止矿工恶意修改区块头中的timestamp
    - 如果区块头的timestamp调成小于正常区块生成的时间，也就越有可能被打包进区块，能挣取更多的挖矿费用
- 3.IsFinalizedTransaction()方法检查区块中每一个交易是否都已经Finalized,此方法后面再分析。
    - 是否Finalized，比较MTP和交易中的LockTime来确定
- 4.对于版本为serializedHeightVersion（2）或更新版本的块，一旦大部分网络已升级，检查coinbase中是否包含了正确的区块高度。
- 5.checkSerializedHeight()方法检查coinbase中的解锁脚本的起始处是否包含了正确的区块高度值。
- 6.检查部署的版本位状态，会在后面再详细分析。


checkBlockHeaderContext()的逻辑：

    func (b *BlockChain) checkBlockHeaderContext(header *wire.BlockHeader, prevNode *blockNode, flags BehaviorFlags) error {
    	fastAdd := flags&BFFastAdd == BFFastAdd
    	if !fastAdd {
    		// 确保区块头中指定的难度与基于前一个块和难度重定向规则的计算难度匹配
    		expectedDifficulty, err := b.calcNextRequiredDifficulty(prevNode,
    			header.Timestamp)
    		if err != nil {
    			return err
    		}
    		blockDifficulty := header.Bits
    		if blockDifficulty != expectedDifficulty {
    			str := "block difficulty of %d is not the expected value of %d"
    			str = fmt.Sprintf(str, blockDifficulty, expectedDifficulty)
    			return ruleError(ErrUnexpectedDifficulty, str)
    		}

    		// 确保在区块头中的时间戳是在最后几个区块的中位时间后面(meidianTimeBlocks)
    		medianTime := prevNode.CalcPastMedianTime()
    		if !header.Timestamp.After(medianTime) {
    			str := "block timestamp of %v is not after expected %v"
    			str = fmt.Sprintf(str, header.Timestamp, medianTime)
    			return ruleError(ErrTimeTooOld, str)
    		}
    	}

    	// 这个块的高度比前面提到的块高一个。
    	blockHeight := prevNode.height + 1

    	// 确保区块链匹配上预定的检查点
    	blockHash := header.BlockHash()
    	if !b.verifyCheckpoint(blockHeight, &blockHash) {
    		str := fmt.Sprintf("block at height %d does not match "+
    			"checkpoint hash", blockHeight)
    		return ruleError(ErrBadCheckpoint, str)
    	}

    	// 在主链分叉前，找到之前的检查点，并防止区块将主链分叉。
    	// 这样可以防止存储新的后者新的区块，这些区块会浪费缓存和磁盘空间。
    	checkpointNode, err := b.findPreviousCheckpoint()
    	if err != nil {
    		return err
    	}
    	if checkpointNode != nil && blockHeight < checkpointNode.height {
    		str := fmt.Sprintf("block at height %d forks the main chain "+
    			"before the previous checkpoint at height %d",
    			blockHeight, checkpointNode.height)
    		return ruleError(ErrForkTooOld, str)
    	}

    	// 当主网已经大部分更新完毕，就拒绝过时的区块版本。这些最初在BIP0034,BIP0065, 和 BIP0066.被投票过。
    	params := b.chainParams
    	if header.Version < 2 && blockHeight >= params.BIP0034Height ||
    		header.Version < 3 && blockHeight >= params.BIP0066Height ||
    		header.Version < 4 && blockHeight >= params.BIP0065Height {

    		str := "new blocks with version %d are no longer valid"
    		str = fmt.Sprintf(str, header.Version)
    		return ruleError(ErrBlockVersionTooOld, str)
    	}
    	return nil
    }

对区块头上下文进行检查的主要逻辑：

- 1.难度调整算法计算链上下一个区块预期的目标难度值，拿这个值与区块头中的难度值进行比较。
    - calcNextRequiredDifficulty(prevNode,header.Timestamp)
    - 如果区块头中的难度不符合预期难度值，就会验证失败，这是为了防止矿工故意选择难度小的值
- 2.计算链上最后一个区块的MTP，用这个值跟当前区块头中的时间戳进行比较。
    - prevNode.CalcPastMedianTime()
    - 如果区块头中的时间戳小于MTP，则验证失败，保证区块的MTP是单调增长的
- 3.将要加入的区块是否对应一个预置的checkpoint点。
    - b.verifyCheckpoint(blockHeight, &blockHash)
    - 如果刚好是预定的检查点，就需要比较Hash是否与预置的Hash值一致，不一致则验证失败，保证了checkpoint点的正确性
- 4.查找区块链上最近的checkpoint点。
    - b.findPreviousCheckpoint()
    - 如果区块的高度小于最近Checkpoint点的高度，则区块试图在检查点之间进行分叉，验证会失败
    - checkpoint点之前分叉的侧链很可能因工作量之和小于主链的工作量之和而没有机会成为主链
- 5.对区块高度和版本号进行检查，对于不同的BIP版本，其区块高度是需要大于某个值的。

总结：checkBlockHeaderContext()是检查区块头中的难度值，时间戳，检查点检查，高度和版本是否符合链上的要求。

其中calcNextRequiredDifficulty()方法就涉及到难度调整算法，看看其是如何实现的：

    func (b *BlockChain) calcNextRequiredDifficulty(lastNode *blockNode, newBlockTime time.Time) (uint32, error) {
    	// Genesis block.
    	if lastNode == nil {
    		return b.chainParams.PowLimitBits, nil
    	}

    	// 如果此块不处于难度重定时间间隔，则返回前一个块的难度要求。
    	if (lastNode.height+1)%b.blocksPerRetarget != 0 {
    		// 对于支持它的网络，在没有挖掘到区块的情况下，一旦经过了太多时间，就可以特别减少所需的难度。
    		if b.chainParams.ReduceMinDifficulty {
    			// 如果在所需时间超过后，还是挖不到一个区块，那就直接调整到最低难度返回。
    			reductionTime := int64(b.chainParams.MinDiffReductionTime /
    				time.Second)
    			allowMinTime := lastNode.timestamp + reductionTime
    			if newBlockTime.Unix() > allowMinTime {
    				return b.chainParams.PowLimitBits, nil
    			}

    			// 如果该区块在期望的时间范围内挖到了，那么返回最后区块的难度，最后区块没有应用特殊的最低难度规则。
    			return b.findPrevTestNetDifficulty(lastNode), nil
    		}

    		// 对于主网络（或任何无法识别的网络），只需返回上一个模块的难度要求即可。
    		return lastNode.bits, nil
    	}

    	// Get the block node at the previous retarget (targetTimespan days worth of blocks).
    	firstNode := lastNode.RelativeAncestor(b.blocksPerRetarget - 1)
    	if firstNode == nil {
    		return 0, AssertError("unable to obtain previous retarget block")
    	}

    	// 限制因为之前难度而出现的难度调整总量
    	actualTimespan := lastNode.timestamp - firstNode.timestamp
    	adjustedTimespan := actualTimespan
    	if actualTimespan < b.minRetargetTimespan {
    		adjustedTimespan = b.minRetargetTimespan
    	} else if actualTimespan > b.maxRetargetTimespan {
    		adjustedTimespan = b.maxRetargetTimespan
    	}

    	// Calculate new target difficulty as:
    	//  currentDifficulty * (adjustedTimespan / targetTimespan)
    	// 结果使用整数除法，这意味着它将略微向下舍入。 Bitcoind也使用整数除法来计算这个结果。
    	oldTarget := CompactToBig(lastNode.bits)
    	newTarget := new(big.Int).Mul(oldTarget, big.NewInt(adjustedTimespan))
    	targetTimeSpan := int64(b.chainParams.TargetTimespan / time.Second)
    	newTarget.Div(newTarget, big.NewInt(targetTimeSpan))

    	// Limit new value to the proof of work limit.
    	if newTarget.Cmp(b.chainParams.PowLimit) > 0 {
    		newTarget.Set(b.chainParams.PowLimit)
    	}

    	// 记录新的目标难度并返回。
    	// 新的目标日志记录有意将这些位转换回数字而不是使用newTarget，因为转换为紧凑表示会失去精度。
    	newTargetBits := BigToCompact(newTarget)
    	log.Debugf("Difficulty retarget at block height %d", lastNode.height+1)
    	log.Debugf("Old target %08x (%064x)", lastNode.bits, oldTarget)
    	log.Debugf("New target %08x (%064x)", newTargetBits, CompactToBig(newTargetBits))
    	log.Debugf("Actual timespan %v, adjusted timespan %v, target timespan %v",
    		time.Duration(actualTimespan)*time.Second,
    		time.Duration(adjustedTimespan)*time.Second,
    		b.chainParams.TargetTimespan)

    	return newTargetBits, nil
    }


calcNextRequiredDifficulty()根据难度重定向规则计算传递前一个块节点之后块所需的难度。
此函数与导出的CalcNextRequiredDifficulty不同，导出的版本使用当前最佳链作为前一个块节点，而此函数接受任何块节点。
其难度调整算法逻辑如下：

- 1.需要加入的区块的高度不处于难度重定时间间隔，则返回父区块的难度值，不进行难度调整。
- 2.在测试网络中，在没有挖掘到区块的情况下，一旦经过了太多时间，就可以特别减少所需的难度。
- 3.计算上一次调整周期内的时长，这个时长的有效范围是3.5天到56天，如果超过范围，则取上限或下限。
- 4.按照公式计算新的难度值
    - currentDifficulty * (adjustedTimespan / targetTimespan)
    - targetTimespan是14天，难度调整算法是为了稳定出块的速度
- 5.如果新的目标难度值大于设定的上限，则将其值设为该上限。
- 6.BigToCompact()方法将目标难度值编码为难度Bits

在通过checkBlockContext()区块上下文检查后，maybeAcceptBlock()将调用connectBestChain()将区块写入区块链。

    func (b *BlockChain) connectBestChain(node *blockNode, block *btcutil.Block, flags BehaviorFlags) (bool, error) {
    	fastAdd := flags&BFFastAdd == BFFastAdd

    	// 最多的情况就是，我们将新区块用来扩展主链
    	parentHash := &block.MsgBlock().Header.PrevBlock
    	if parentHash.IsEqual(&b.bestChain.Tip().hash) {
    		// Skip checks if node has already been fully validated.
    		fastAdd = fastAdd || b.index.NodeStatus(node).KnownValid()

    		// 在不违反任何规则，也不需要实际连接到区块的情况下，执行一些检查来验证区块可以连接到主链。
    		view := NewUtxoViewpoint()
    		view.SetBestHash(parentHash)
    		stxos := make([]spentTxOut, 0, countSpentOutputs(block))
    		if !fastAdd {
    			err := b.checkConnectBlock(node, block, view, &stxos)
    			if err == nil {
    				b.index.SetStatusFlags(node, statusValid)
    			} else if _, ok := err.(RuleError); ok {
    				b.index.SetStatusFlags(node, statusValidateFailed)
    			} else {
    				return false, err
    			}

    			// 故意忽略掉在将节点状态更新写入数据库的错误。如果写入失败，那也不是世界末日。
    			// 如果区块有效，将使用connectBlock进行刷新，如果区块无效，可能发生的最坏情况就是我们在重新启动后重新验证块。
    			if writeErr := b.index.flushToDB(); writeErr != nil {
    				log.Warnf("Error flushing block index changes to disk: %v",
    					writeErr)
    			}

    			if err != nil {
    				return false, err
    			}
    		}

    		// 在快速添加的情况下，调过了检查区块连接的代码，
    		// 因此utxo视图需要加载引用的utxos，并将这个区块创建的新的utxos进行花费和添加。
    		if fastAdd {
    			err := view.fetchInputUtxos(b.db, block)
    			if err != nil {
    				return false, err
    			}
    			err = view.connectTransactions(block, &stxos)
    			if err != nil {
    				return false, err
    			}
    		}

    		// Connect the block to the main chain.
    		err := b.connectBlock(node, block, view, stxos)
    		if err != nil {
    			return false, err
    		}

    		return true, nil
    	}
    	if fastAdd {
    		log.Warnf("fastAdd set in the side chain case? %v\n",
    			block.Hash())
    	}

    	// 我们正在扩展（或创造）一个侧链，但这个新的侧链的累积工作还不足以使其成为新的链条。
    	if node.workSum.Cmp(b.bestChain.Tip().workSum) <= 0 {
    		// Log information about how the block is forking the chain.
    		fork := b.bestChain.FindFork(node)
    		if fork.hash.IsEqual(parentHash) {
    			log.Infof("FORK: Block %v forks the chain at height %d"+
    				"/block %v, but does not cause a reorganize",
    				node.hash, fork.height, fork.hash)
    		} else {
    			log.Infof("EXTEND FORK: Block %v extends a side chain "+
    				"which forks the chain at height %d/block %v",
    				node.hash, fork.height, fork.hash)
    		}

    		return false, nil
    	}

    	// 我们正在扩展（或创建）一个侧链，这个新的侧链的累积工作不仅仅是旧的最佳链，所以这个侧链需要成为主链。
    	// 为了做到这一点，找到叉子两侧的共同祖先，
    	// 从主链中断开形成（现在）旧叉子的块，并将形成新链的块连接到主链上从公共点开始ancenstor（分叉点）。
    	detachNodes, attachNodes := b.getReorganizeNodes(node)

    	// Reorganize the chain.
    	log.Infof("REORGANIZE: Block %v is causing a reorganize.", node.hash)
    	err := b.reorganizeChain(detachNodes, attachNodes)

    	// getReorganizeNodes或reorganizeChain可能会对块索引进行未保存的更改，因此无论是否存在错误都需要刷新。
    	// 索引只会在块连接失败时变脏，所以我们可以忽略任何写入错误。
    	if writeErr := b.index.flushToDB(); writeErr != nil {
    		log.Warnf("Error flushing block index changes to disk: %v", writeErr)
    	}

    	return err == nil, err
    }

connectBestChain处理将传递的块连接到链，同时根据工作证明的链条考虑正确的链选择。
在典型的情况下，新的区块只是扩展了主链。
然而，它也可能扩展（或创造）侧链（叉），这可能或不可能最终成为主链，这取决于哪个叉累积地具有最多的工作证明。

connectBestChain()的输入参数是将要加入到区块链的区块的blockNode对象和btcutil.Block辅助对象。输出的第一个结果指明是否添加到主链。

- 1.如果父区块的Hash就是区块链中尾巴tip的值，则区块将被写入主链。
    - if parentHash.IsEqual(&b.bestChain.Tip().hash)
- 2.创建一个UtxoViewpoint对象，并将view的观察点设为主链的尾巴tip，UtxoViewpoint表示区块链上从创世区块到尾巴的所有包含UTXO的交易。
    - view := NewUtxoViewpoint()
    - view.SetBestHash(parentHash)
    - UTXO（s）将用来验证重复交易，双重支付等
- 3.countSpentOutputs()计算区块内所有交易花费的交易输出的个数，也就是说总的输入个数，TxInput的总个数，创建一个容量为总个数的spentTxOut slice。
    - stxos := make([]spentTxOut, 0, countSpentOutputs(block))
- 4.checkConnectBlock()对区块内的交易做验证，并更新utxoset，接下来还会深入分析。
    - err := b.checkConnectBlock(node, block, view, &stxos)
- 5.交易验证通过后，connectBlock()将区块相关的信息写入数据库，真正实现将区块写入主链。
    - err := b.connectBlock(node, block, view, stxos)
- 6.如果父区块的Hash不等于主链尾巴tip值，则将区块写入侧链，根据主侧链的工作量和，主侧有可能对调。
- 7.如果侧链的工作量和小于主链的工作量和，直接返回。
    - if node.workSum.Cmp(b.bestChain.Tip().workSum) <= 0
    - 如果父区块在主链上，则从当前区块开始分叉 else{
    - 如果父区块在侧链上，则扩展侧链  if fork.hash.IsEqual(parentHash) {
- 8.如果侧链的工作量和大于主链的工作量和，则需要对调主侧链。
    - getReorganizeNodes()找到分叉点以及侧链和主链上的区块节点
    - detachNodes, attachNodes := b.getReorganizeNodes(node)
- 9.reorganizeChain()实现侧链变主链。

在checkConnectBlock()和connectBlock()中对交易验证或者utxoset更新时都要对UtxoViewpoint的操作：

    // 记录主链上交易的Hash与对应的*UtxoEntry之间的map集合
    type UtxoViewpoint struct {
    	entries  map[wire.OutPoint]*UtxoEntry
    	bestHash chainhash.Hash
    }

    // UtxoEntry代表一个包含了Utxo的交易
    type UtxoEntry struct {
    	amount      int64   // utxo的输出币值
    	pkScript    []byte // 输入的锁定脚本
    	blockHeight int32  // UtxoEntry对应的交易所在的区块的高度
    	packedFlags txoFlags
    }

区块要上主链，那么区块中的所有交易产生的UTXO将被加入到UtxoViewpoint对应的utxo集合中，
交易的输入将从utxo集合中移除，主链的状态与utxo集合的状态是紧密联系的，区块链状态的一致性实质上指的是主链和utxo集合的状态的一致性。

reorganize()方法将侧链变成主链，区块将被从之前主链上移除，这是包含的所有交易中已经花费的交易输出将重新变成utxo，并进入utxo集合中。

区块在加入主链时，包含的交易的所有花费的spentTxOut将会被记录下来，压缩后写入数据库。

下面这图能直观理解utxo跟主链侧链的关系。

![]()

基于上图，进行重复交易和双发的验证的思路探索一下。

- 前提是区块要加入主链，主链上的有些交易输出已经被区块中的交易花费掉了，那些还没有被完全花费的交易在utxoset中通过untxentry记录。
- 新区块中有无交易和主链上的交易重复，检查区块中的交易是否已经在主链上来实现。但主链上交易太多，比对耗时，只验证新区块中的交易是否在utxoset中。
    - 只验证交易是否在utxoset中是合理的，utxoentry指向的交易肯定在主链上，而不在utxoset中的交易已经被完全花费
- 验证双花，通过验证新区块中的交易输入是否指向了utxoset中的utxoentry来实现。
    - 如果交易输入未指向utxoset中任何记录，说明试图花费一个已经花费了的交易
    - 如果交易输入指向utxoset中的utxoentry，但已经被新区块中排在前面的交易花费掉了，或者被矿工正在挖的区块里的交易花费掉了，则还是双发交易
    - 当交易输入指向utxoset中utxoentry，且未被花费才能通过双发检查。

对区块内的交易进行验证的方法checkConnectBlock()：

    // checkConnectBlock执行多次验证，确保将合格的区块连接到合格的视图呈现的链上时不违反任何规则。
    // 更新合格的视图将花费所有引用的输出并添加由区块创建的所有新utxos。
    // 该视图将表示链的状态，就好像区块实际连接一样，因此视图的最佳hash也会更新为通过的区块。

    // 一个检查验证的例子：确保区块连接不会引起任何重复交易hashes，
    // 这些老的交易可能是没有全部支付，可能是双重花费，可能超过了每个区块允许的最大签名操作，可能因为预期的区块subsidy而产生无效值，可能因为交易脚本验证失败。



    func (b *BlockChain) checkConnectBlock(node *blockNode, block *btcutil.Block, view *UtxoViewpoint, stxos *[]spentTxOut) error {
    	// 如果侧链块在数据库中结束，则在此之前调用CheckBlockSanity以防先前版本允许不再有效的块。
    	// 但是，由于该实施方案目前仅使用侧链块的存储器，所以目前不需要。

    	// The coinbase for the Genesis block is not spendable, so just return an error now.
    	if node.hash.IsEqual(b.chainParams.GenesisHash) {
    		str := "the coinbase for the genesis block is not spendable"
    		return ruleError(ErrMissingTxOut, str)
    	}

    	// Ensure the view is for the node being checked.
    	parentHash := &block.MsgBlock().Header.PrevBlock
    	if !view.BestHash().IsEqual(parentHash) {
    		return AssertError(fmt.Sprintf("inconsistent view when "+
    			"checking block connection: best hash is %v instead "+
    			"of expected %v", view.BestHash(), parentHash))
    	}

    	// BIP0030 added a rule to prevent blocks which contain duplicate
    	// transactions that 'overwrite' older transactions which are not fully
    	// spent.  See the documentation for checkBIP0030 for more details.
    	//
    	// There are two blocks in the chain which violate this rule, so the
    	// check must be skipped for those blocks.  The isBIP0030Node function
    	// is used to determine if this block is one of the two blocks that must
    	// be skipped.
    	//
    	// In addition, as of BIP0034, duplicate coinbases are no longer
    	// possible due to its requirement for including the block height in the
    	// coinbase and thus it is no longer possible to create transactions
    	// that 'overwrite' older ones.  Therefore, only enforce the rule if
    	// BIP0034 is not yet active.  This is a useful optimization because the
    	// BIP0030 check is expensive since it involves a ton of cache misses in
    	// the utxoset.
    	if !isBIP0030Node(node) && (node.height < b.chainParams.BIP0034Height) {
    		err := b.checkBIP0030(node, block, view)
    		if err != nil {
    			return err
    		}
    	}

    	// 加载区块中所有交易的交易输入所引用的utxos，这些utxos没有存在于数据库的utxo视图中。

    	// 这些utxo条目用于验证交易，比如交易输入，计算付费脚本哈希和脚本。
    	err := view.fetchInputUtxos(b.db, block)
    	if err != nil {
    		return err
    	}

    	// BIP0016 describes a pay-to-script-hash type that is considered a
    	// "standard" type.  The rules for this BIP only apply to transactions
    	// after the timestamp defined by txscript.Bip16Activation.  See
    	// https://en.bitcoin.it/wiki/BIP_0016 for more details.
    	enforceBIP0016 := node.timestamp >= txscript.Bip16Activation.Unix()

    	// 查询Segwit软叉部署的版本位状态。如果segwit处于活动状态，我们将切换到执行所有新规则。
    	segwitState, err := b.deploymentState(node.parent, chaincfg.DeploymentSegwit)
    	if err != nil {
    		return err
    	}
    	enforceSegWit := segwitState == ThresholdActive

    	// 签名操作的数量必须少于每个块允许的最大数量。
    	// 请注意，块上的初步健全性检查还包括与此类似的检查，
    	// 但此检查会扩展计数以在每个输入交易公钥脚本中包含付费脚本哈希签名操作的精确计数。
    	transactions := block.Transactions()
    	totalSigOpCost := 0
    	for i, tx := range transactions {
    		// 由于第一个（也是唯一的第一个）事务已经被验证为coinbase事务，所以使用i == 0作为countP2SHSigOps标志的优化，
    		// 以确定交易是否是coinbase交易，而不是必须完成coinbase检查再次
    		sigOpCost, err := GetSigOpCost(tx, i == 0, view, enforceBIP0016,
    			enforceSegWit)
    		if err != nil {
    			return err
    		}

    		// 检查溢出或超出限制。我们必须在每次循环迭代时都这样做，以避免溢出。
    		lastSigOpCost := totalSigOpCost
    		totalSigOpCost += sigOpCost
    		if totalSigOpCost < lastSigOpCost || totalSigOpCost > MaxBlockSigOpsCost {
    			str := fmt.Sprintf("block contains too many "+
    				"signature operations - got %v, max %v",
    				totalSigOpCost, MaxBlockSigOpsCost)
    			return ruleError(ErrTooManySigOps, str)
    		}
    	}

    	// 对每笔交易的输入进行几次检查。也累计总费用。
        // 这在技术上可以与上面的循环结合使用，而不是在事务上运行另一个循环，但通过分离它，
        // 我们可以避免在签名操作时针对所有输入运行更昂贵的检查（与运行脚本相比仍然相对便宜）从而超出界限。
    	var totalFees int64
    	for _, tx := range transactions {
    		txFee, err := CheckTransactionInputs(tx, node.height, view,
    			b.chainParams)
    		if err != nil {
    			return err
    		}

    		// 总费用，并确保我们不会溢出累加器。
    		lastTotalFees := totalFees
    		totalFees += txFee
    		if totalFees < lastTotalFees {
    			return ruleError(ErrBadFees, "total fees for block "+
    				"overflows accumulator")
    		}

    		// 添加这个交易的所有输出，这些输出不可证明是可用的utxos。
    		// 此外，传递的花费txos片段更新为包含每个花费txout的条目，每个交易花费在它们的花费中。
    		err = view.connectTransaction(tx, node.height, stxos)
    		if err != nil {
    			return err
    		}
    	}

    	// 基础交易的总产值不得超过预期补贴价值加上开采区块所获得的交易总费用。
    	// 这里忽略溢出和超出范围错误是安全的，因为这些错误条件已经被checkTransactionSanity捕获。
    	var totalSatoshiOut int64
    	for _, txOut := range transactions[0].MsgTx().TxOut {
    		totalSatoshiOut += txOut.Value
    	}
    	expectedSatoshiOut := CalcBlockSubsidy(node.height, b.chainParams) +
    		totalFees
    	if totalSatoshiOut > expectedSatoshiOut {
    		str := fmt.Sprintf("coinbase transaction for block pays %v "+
    			"which is more than expected value of %v",
    			totalSatoshiOut, expectedSatoshiOut)
    		return ruleError(ErrBadCoinbaseValue, str)
    	}

    	// 由于通过检查点验证了有效性（所有事务都包含在Merkle根散列中，因此下一个检查点将检测到任何更改），
        // 因此如果此节点位于最新的已知检查点之前，请不要运行脚本。这是一个巨大的优化，因为运行脚本是块处理中最耗时的部分。
    	checkpoint := b.LatestCheckpoint()
    	runScripts := true
    	if checkpoint != nil && node.height <= checkpoint.Height {
    		runScripts = false
    	}

    	// Blocks created after the BIP0016 activation time need to have the
    	// pay-to-script-hash checks enabled.
    	var scriptFlags txscript.ScriptFlags
    	if enforceBIP0016 {
    		scriptFlags |= txscript.ScriptBip16
    	}

    	// Enforce DER signatures for block versions 3+ once the historical
    	// activation threshold has been reached.  This is part of BIP0066.
    	blockHeader := &block.MsgBlock().Header
    	if blockHeader.Version >= 3 && node.height >= b.chainParams.BIP0066Height {
    		scriptFlags |= txscript.ScriptVerifyDERSignatures
    	}

    	// Enforce CHECKLOCKTIMEVERIFY for block versions 4+ once the historical
    	// activation threshold has been reached.  This is part of BIP0065.
    	if blockHeader.Version >= 4 && node.height >= b.chainParams.BIP0065Height {
    		scriptFlags |= txscript.ScriptVerifyCheckLockTimeVerify
    	}

    	// Enforce CHECKSEQUENCEVERIFY during all block validation checks once
    	// the soft-fork deployment is fully active.
    	csvState, err := b.deploymentState(node.parent, chaincfg.DeploymentCSV)
    	if err != nil {
    		return err
    	}
    	if csvState == ThresholdActive {
    		// If the CSV soft-fork is now active, then modify the
    		// scriptFlags to ensure that the CSV op code is properly
    		// validated during the script checks bleow.
    		scriptFlags |= txscript.ScriptVerifyCheckSequenceVerify

    		// We obtain the MTP of the *previous* block in order to
    		// determine if transactions in the current block are final.
    		medianTime := node.parent.CalcPastMedianTime()

    		// Additionally, if the CSV soft-fork package is now active,
    		// then we also enforce the relative sequence number based
    		// lock-times within the inputs of all transactions in this
    		// candidate block.
    		for _, tx := range block.Transactions() {
    			// A transaction can only be included within a block
    			// once the sequence locks of *all* its inputs are
    			// active.
    			sequenceLock, err := b.calcSequenceLock(node, tx, view,
    				false)
    			if err != nil {
    				return err
    			}
    			if !SequenceLockActive(sequenceLock, node.height,
    				medianTime) {
    				str := fmt.Sprintf("block contains " +
    					"transaction whose input sequence " +
    					"locks are not met")
    				return ruleError(ErrUnfinalizedTx, str)
    			}
    		}
    	}

    	// Enforce the segwit soft-fork package once the soft-fork has shifted
    	// into the "active" version bits state.
    	if enforceSegWit {
    		scriptFlags |= txscript.ScriptVerifyWitness
    		scriptFlags |= txscript.ScriptStrictMultiSig
    	}

    	// Now that the inexpensive checks are done and have passed, verify the
    	// transactions are actually allowed to spend the coins by running the
    	// expensive ECDSA signature check scripts.  Doing this last helps
    	// prevent CPU exhaustion attacks.
    	if runScripts {
    		err := checkBlockScripts(block, view, scriptFlags, b.sigCache,
    			b.hashCache)
    		if err != nil {
    			return err
    		}
    	}

    	// Update the best hash for view to include this block since all of its
    	// transactions have been connected.
    	view.SetBestHash(&node.hash)

    	return nil
    }

checkConnectBlock()是区块加入主链前最后也是最复杂的检查，其主要逻辑如下：

- 1.通过交易hash值来检查区块中是否有跟主链中相同的交易。
    - 交易hash是如何得到的？整个交易结构序列化后进行两次SHA256()的结果。
    - 判断是否要进行BIP30检查，已经部署了BIP34的区块进行。
    - checkBIP0030()，当区块中的所有交易不再utxoset中或者对应的utxoentry的所有output都已经花费掉，就通过检查。
- 2.UtxoViewpoint的fetchInputUtxos()将区块中所有交易的输入引用到的utxo从db加载到内存中。
- 3.判断区块是否支持P2SH(pay to Script Hash)。P2SH是一种交易脚本类型，例如，对于一个签名的交易，其解锁和锁定脚本有固定的形式：

    scriptSig: [signature] {[pubkey] OP_CHECKSIG}
    scriptPubKey: OP_HASH160 [20-byte-hash of {[pubkey] OP_CHECKSIG} ] OP_EQUAL

    - 同时也需要判断每个区块中的脚本操作符总个数不能超过20000个，checkBlockSanity()中只统计了区块交易中的锁定脚本和解锁脚本中操作符的总个数，
    - 对于P2SH脚本来说，解锁脚本中还包含了序列化的赎回脚本，因而还需要统计赎回脚本中的操作符个数。

- 4.CheckTransactionInputs()检查双重支付，交易的输出额是否超过输入额，还有就是coinbase交易能否被花费等，还要计算交易的费用。此方法等会还会继续分析。
- 5.UtxoViewpoint的connectTransaction()方法，将交易中的输入utxo标记为已花费，并更新传入的slice stxos，将交易的输出添加到utxoset中。
    - spentTxOutputs和utxoset中已经花费的utxoentry还没有最终更新，要connectBlock()将区块最终连入主链时更新并将最新状态写入数据库。
- 6.检查coinbase交易的输出总值是否超过预期值，防止矿工随意伪造奖励。
    - coinbase交易的输出是对矿工挖矿的奖励。
    - 预期值是挖矿的补贴和区块中所有交易的费用之和。
    - 补贴是每4年减半
- 7.准备对交易中的脚本进行验证，这是相对耗时的操作
    - 如果区块的高度在预置的最新Checkpoints区块之下，那么可以跳过脚本检查，直接将错误发现推迟到下一个Checkpoint区块检查点。
    - 如果交易有变化，其Hash变化，其区块Hash也会变化，进而影响Checkpoint区块的Hash，checkBlockHeaderContext()中的Checkpoint验证会失败。
    - 区块要有足够的确认才能成为Checkpoint，Btcd版本中，2016个确认才能成为Checkpoint。
- 8. deploymentState()查询CSV的部署状态。
    - 已经部署，calcSequenceLock()，根据交易输入的Sequence值计算交易相对LockTime值。
    - SequenceLockActive()检查区块的MTP和高度是否已经超过交易的锁定时间和锁定高度。
    - 区块的高度小于交易锁定高度或者区块的MTP小于交易锁定时间，则交易不应该被打包进该区块。
- 9. 如果区块在最新的Checkpoint区块之后，就需要进行脚本检验，checkBlockScript()方法来进行。
- 10.将UtxoViewpoint的观察点更新为当前区块，随后该区块的相关状态会最终写入区块链。

总结：从connectBestChain()的实现，checkConnectBlock()通过以后，调用connectBlock()将区块最终写入主链并将相关状态写入数据库。
接下来还要重点分析的：

- 检查交易输入，更新utxoset，检查交易的相对锁定时间
- getReoriganizedNodes()和reorganizeChain()将侧链变主链的实现

接下来就是分析checkConnectBlock()里面的一些方法，看看具体是如何将区块最终写入主链，还有在此之前的一系列复杂的检查验证。

    func CheckTransactionInputs(tx *btcutil.Tx, txHeight int32, utxoView *UtxoViewpoint, chainParams *chaincfg.Params) (int64, error) {
    	// Coinbase transactions have no inputs.
    	if IsCoinBase(tx) {
    		return 0, nil
    	}

    	txHash := tx.Hash()
    	var totalSatoshiIn int64
    	for txInIndex, txIn := range tx.MsgTx().TxIn {
    		// Ensure the referenced input transaction is available.
    		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
    		if utxo == nil || utxo.IsSpent() {
    			str := fmt.Sprintf("output %v referenced from "+
    				"transaction %s:%d either does not exist or "+
    				"has already been spent", txIn.PreviousOutPoint,
    				tx.Hash(), txInIndex)
    			return 0, ruleError(ErrMissingTxOut, str)
    		}

    		// Ensure the transaction is not spending coins which have not
    		// yet reached the required coinbase maturity.
    		if utxo.IsCoinBase() {
    			originHeight := utxo.BlockHeight()
    			blocksSincePrev := txHeight - originHeight
    			coinbaseMaturity := int32(chainParams.CoinbaseMaturity)
    			if blocksSincePrev < coinbaseMaturity {
    				str := fmt.Sprintf("tried to spend coinbase "+
    					"transaction output %v from height %v "+
    					"at height %v before required maturity "+
    					"of %v blocks", txIn.PreviousOutPoint,
    					originHeight, txHeight,
    					coinbaseMaturity)
    				return 0, ruleError(ErrImmatureSpend, str)
    			}
    		}

    		// 确保交易金额在范围内。
    		// 输入交易的每个输出值不得大于或等于每个交易所允许的最大值。交易中的所有金额均以称为satoshi的单位值计算
    		originTxSatoshi := utxo.Amount()
    		if originTxSatoshi < 0 {
    			str := fmt.Sprintf("transaction output has negative "+
    				"value of %v", btcutil.Amount(originTxSatoshi))
    			return 0, ruleError(ErrBadTxOutValue, str)
    		}
    		if originTxSatoshi > btcutil.MaxSatoshi {
    			str := fmt.Sprintf("transaction output value of %v is "+
    				"higher than max allowed value of %v",
    				btcutil.Amount(originTxSatoshi),
    				btcutil.MaxSatoshi)
    			return 0, ruleError(ErrBadTxOutValue, str)
    		}

    		// 所有输出的总数不得超过每笔交易允许的最大数量。另外，我们可能会溢出累加器，检查溢出。
    		lastSatoshiIn := totalSatoshiIn
    		totalSatoshiIn += originTxSatoshi
    		if totalSatoshiIn < lastSatoshiIn ||
    			totalSatoshiIn > btcutil.MaxSatoshi {
    			str := fmt.Sprintf("total value of all transaction "+
    				"inputs is %v which is higher than max "+
    				"allowed value of %v", totalSatoshiIn,
    				btcutil.MaxSatoshi)
    			return 0, ruleError(ErrBadTxOutValue, str)
    		}
    	}

    	// 计算此次交易的总产量。
    	// 这里忽略溢出和超出范围错误是安全的，因为这些错误条件已经被checkTransactionSanity捕获。
    	var totalSatoshiOut int64
    	for _, txOut := range tx.MsgTx().TxOut {
    		totalSatoshiOut += txOut.Value
    	}

    	// Ensure the transaction does not spend more than its inputs.
    	if totalSatoshiIn < totalSatoshiOut {
    		str := fmt.Sprintf("total value of all transaction inputs for "+
    			"transaction %v is %v which is less than the amount "+
    			"spent of %v", txHash, totalSatoshiIn, totalSatoshiOut)
    		return 0, ruleError(ErrSpendTooHigh, str)
    	}

    	// NOTE: bitcoind checks if the transaction fees are < 0 here, but that
    	// is an impossible condition because of the check above that ensures
    	// the inputs are >= the outputs.
    	txFeeInSatoshi := totalSatoshiIn - totalSatoshiOut
    	return txFeeInSatoshi, nil
    }

如果对交易进行检查验证的逻辑：

- 1.查看是否为coinbase交易，如果是就直接返回，因为没有有效输入。
- 2.检查交易中每一个交易输入。
    - 先从utxoset中找交易输入的交易是否已经存在了。
    - UtxoViewpoint的fetchInputUtxos()方法将区块中所有交易花费的输入都加载到内存中。
    - 交易输入不再utxoset中，可能是试图花费一个无效的交易输入或者这个交易输入已经被花费掉了，当然不能通过验证。
- 3.交易花费的是一个coinbase交易，则需要检查coinbase交易是否已经成熟。
    - 检查coinbase交易是否成熟：当前区块的高度减去coinbase生成的区块是否大于设定的CoinbaseMaturity值。
    - coinbase至少需要CoinbaseMaturity个确认才能被花费
- 4.检查交易输入是否已经花费，是否存在双重支付。
- 5.检查交易花费的utxo的输出币值是否在0-2100万BTC之间。
- 6.所有输出的币值综合，不能大于所花费的utxo币值总和。
- 7.计算交易费用，费用大于零。
    - 交易的总输入和交易总输出币值的差就是交易费用

很明显，关于交易的检查核心就是utxoset，utxoset中的utxoentry在区块加入区块链时写入数据库，
当需要访问utxoentry的时候，在从数据库中加载出来，在checkConnectBlock()方法中，可以从UtxoViewpoint的fetchInputUtxos()方法分析utxo的存取：

    func (view *UtxoViewpoint) fetchInputUtxos(db database.DB, block *btcutil.Block) error {
    	// Build a map of in-flight transactions because some of the inputs in
    	// this block could be referencing other transactions earlier in this
    	// block which are not yet in the chain.
    	txInFlight := map[chainhash.Hash]int{}
    	transactions := block.Transactions()
    	for i, tx := range transactions {
    		txInFlight[*tx.Hash()] = i
    	}

    	// Loop through all of the transaction inputs (except for the coinbase
    	// which has no inputs) collecting them into sets of what is needed and
    	// what is already known (in-flight).
    	neededSet := make(map[wire.OutPoint]struct{})
    	for i, tx := range transactions[1:] {
    		for _, txIn := range tx.MsgTx().TxIn {
    			// It is acceptable for a transaction input to reference
    			// the output of another transaction in this block only
    			// if the referenced transaction comes before the
    			// current one in this block.  Add the outputs of the
    			// referenced transaction as available utxos when this
    			// is the case.  Otherwise, the utxo details are still
    			// needed.
    			//
    			// NOTE: The >= is correct here because i is one less
    			// than the actual position of the transaction within
    			// the block due to skipping the coinbase.
    			originHash := &txIn.PreviousOutPoint.Hash
    			if inFlightIndex, ok := txInFlight[*originHash]; ok &&
    				i >= inFlightIndex {

    				originTx := transactions[inFlightIndex]
    				view.AddTxOuts(originTx, block.Height())
    				continue
    			}

    			// Don't request entries that are already in the view
    			// from the database.
    			if _, ok := view.entries[txIn.PreviousOutPoint]; ok {
    				continue
    			}

    			neededSet[txIn.PreviousOutPoint] = struct{}{}
    		}
    	}

    	// Request the input utxos from the database.
    	return view.fetchUtxosMain(db, neededSet)
    }

fetchInputUtxos()将区块中所有交易的输入utxo加载到内存中，其主要逻辑：

- 1.遍历区块，记录区块中所有交易的序号。
- 2.遍历除coinbase交易外的所有其他交易，进而遍历每个交易中的所有交易输入。
    - 如果交易花费的是当前区块中排在前面的某一个交易，则将花费的交易加入到utxoset中
    - 如果交易的输入utxo已经在utxoset中，则继续遍历剩下的交易输入
- 3.将所有花费的且不在utxoset中的交易的Hash记录到txNeededSet中，准备在db中根据Hash查找utxoentry。
- 4.调用fetchUtxoMain从数据库中查询utxoentry，并加载到utxoset中。

fetchUtxoMain()：

    func (view *UtxoViewpoint) fetchUtxosMain(db database.DB, outpoints map[wire.OutPoint]struct{}) error {
    	// Nothing to do if there are no requested outputs.
    	if len(outpoints) == 0 {
    		return nil
    	}

    	// Load the requested set of unspent transaction outputs from the point
    	// of view of the end of the main chain.
    	//
    	// NOTE: Missing entries are not considered an error here and instead
    	// will result in nil entries in the view.  This is intentionally done
    	// so other code can use the presence of an entry in the store as a way
    	// to unnecessarily avoid attempting to reload it from the database.
    	return db.View(func(dbTx database.Tx) error {
    		for outpoint := range outpoints {
    			entry, err := dbFetchUtxoEntry(dbTx, outpoint)
    			if err != nil {
    				return err
    			}
    			view.entries[outpoint] = entry
    		}

    		return nil
    	})
    }

其逻辑是通过db的View()方法获取数据库的只读Transaction，然后调用dbFetchUtxoEntry(dbTx, outpoint)来查找具体的outpoint。

    func dbFetchUtxoEntry(dbTx database.Tx, outpoint wire.OutPoint) (*UtxoEntry, error) {
    	// Fetch the unspent transaction output information for the passed
    	// transaction output.  Return now when there is no entry.
    	key := outpointKey(outpoint)
    	utxoBucket := dbTx.Metadata().Bucket(utxoSetBucketName)
    	serializedUtxo := utxoBucket.Get(*key)
    	recycleOutpointKey(key)
    	if serializedUtxo == nil {
    		return nil, nil
    	}

    	// A non-nil zero-length entry means there is an entry in the database
    	// for a spent transaction output which should never be the case.
    	if len(serializedUtxo) == 0 {
    		return nil, AssertError(fmt.Sprintf("database contains entry "+
    			"for spent tx output %v", outpoint))
    	}

    	// Deserialize the utxo entry and return it.
    	entry, err := deserializeUtxoEntry(serializedUtxo)
    	if err != nil {
    		// 确保将任何反序列化错误作为数据库损坏错误返回。
    		if isDeserializeErr(err) {
    			return nil, database.Error{
    				ErrorCode: database.ErrCorruption,
    				Description: fmt.Sprintf("corrupt utxo entry "+
    					"for %v: %v", outpoint, err),
    			}
    		}
    		return nil, err
    	}
    	return entry, nil
    }

dbFetchUtxoEntry使用现有的数据库交易从utxo集中获取指定的交易输出。
所有的utxoentry是存在utxoset的Bucket中，Key是交易Hash，Value是utxoetnry的序列化结果。
当区块写入主链或者从主链中被移除时，节点将通过dbPutUtxoView()来更新Bucket中的utxoentry记录。

那些交易输入被花费掉后的utxo需要标记为spent，同时交易输出产生的utxo需要添加到utxoset中。这些都是通过connectTransaction()来实现。

    func (view *UtxoViewpoint) connectTransaction(tx *btcutil.Tx, blockHeight int32, stxos *[]spentTxOut) error {
    	// Coinbase transactions don't have any inputs to spend.
    	if IsCoinBase(tx) {
    		// Add the transaction's outputs as available utxos.
    		view.AddTxOuts(tx, blockHeight)
    		return nil
    	}

    	// Spend the referenced utxos by marking them spent in the view and,
    	// if a slice was provided for the spent txout details, append an entry
    	// to it.
    	for _, txIn := range tx.MsgTx().TxIn {
    		// Ensure the referenced utxo exists in the view.  This should
    		// never happen unless there is a bug is introduced in the code.
    		entry := view.entries[txIn.PreviousOutPoint]
    		if entry == nil {
    			return AssertError(fmt.Sprintf("view missing input %v",
    				txIn.PreviousOutPoint))
    		}

    		// Only create the stxo details if requested.
    		if stxos != nil {
    			// Populate the stxo details using the utxo entry.
    			var stxo = spentTxOut{
    				amount:     entry.Amount(),
    				pkScript:   entry.PkScript(),
    				height:     entry.BlockHeight(),
    				isCoinBase: entry.IsCoinBase(),
    			}
    			*stxos = append(*stxos, stxo)
    		}

    		// Mark the entry as spent.  This is not done until after the
    		// relevant details have been accessed since spending it might
    		// clear the fields from memory in the future.
    		entry.Spend()
    	}

    	// Add the transaction's outputs as available utxos.
    	view.AddTxOuts(tx, blockHeight)
    	return nil
    }

connectTransction通过下面的操作来更新视图：

- 添加所有那些由通过验证的交易的新utxos
- 标记所有那些已经被花费掉的交易的utxos
- 当stxos参数不为空的时候，视图将被更新为每个用过的txout添加一个条目
- 如果视图不包含所需的utxos，则会返回错误

connectTransaction()的主要逻辑：

- 1.如果交易是coinbase交易，则不用处理其输入，直接调用AddTxOuts将其输出添加到utxoset中。
- 2.遍历交易输入，从视图中找到utxoentry。
    - for _, txIn := range tx.MsgTx().TxIn
    - entry := view.entries[txIn.PreviousOutPoint]
- 3.判断stxos是否为nil，根据花费的utxo构造spentTxOut，按照交易的输入的顺序将spentTxOut添加到stxos中。
    - stxos按照顺序记录了交易中所有花费的utxos，并进而按区块中交易的顺序记录了所有交易花费的utxos
    - stxos按照交易及交易输入的顺序记录区块中交易花费的所有utxos
    - 如果区块在分叉链而被主链移除，stxos中的记录将会被追回到utxoset中
    - stoxs也将会被存储到数据库中
    - utxoentry若被完全花费，也就是其所有输出均被花费，将被从utxoset中移除
    - 如果被移除的utxoentry需要恢复，则需要记录区块高度height和isCoinbase字段
- 4.将交易的所有输出添加到utxoset中。
    - view.AddTxOuts(tx, blockHeight)

AddTxOuts()的实现：

    func (view *UtxoViewpoint) AddTxOuts(tx *btcutil.Tx, blockHeight int32) {
    	// Loop all of the transaction outputs and add those which are not
    	// provably unspendable.
    	isCoinBase := IsCoinBase(tx)
    	prevOut := wire.OutPoint{Hash: *tx.Hash()}
    	for txOutIdx, txOut := range tx.MsgTx().TxOut {
    		// Update existing entries.  All fields are updated because it's
    		// possible (although extremely unlikely) that the existing
    		// entry is being replaced by a different transaction with the
    		// same hash.  This is allowed so long as the previous
    		// transaction is fully spent.
    		prevOut.Index = uint32(txOutIdx)
    		view.addTxOut(prevOut, txOut, isCoinBase, blockHeight)
    	}
    }

AddTxOuts在通过验证的交易中添加所有交易输出，这些交易输出对于视图来说是不可证实的。

当视图已经有任何交易输出的条目时，它们只是标记为未使用。

所有字段都会针对现有条目进行更新，因为它可能在重组期间发生了变化。

AddTxOuts()的主要逻辑是遍历TxOut，然后调用addTxOut

    func (view *UtxoViewpoint) addTxOut(outpoint wire.OutPoint, txOut *wire.TxOut, isCoinBase bool, blockHeight int32) {
    	// Don't add provably unspendable outputs.
    	if txscript.IsUnspendable(txOut.PkScript) {
    		return
    	}

    	// Update existing entries.  All fields are updated because it's
    	// possible (although extremely unlikely) that the existing entry is
    	// being replaced by a different transaction with the same hash.  This
    	// is allowed so long as the previous transaction is fully spent.
    	entry := view.LookupEntry(outpoint)
    	if entry == nil {
    		entry = new(UtxoEntry)
    		view.entries[outpoint] = entry
    	}

    	entry.amount = txOut.Value
    	entry.pkScript = txOut.PkScript
    	entry.blockHeight = blockHeight
    	entry.packedFlags = tfModified
    	if isCoinBase {
    		entry.packedFlags |= tfCoinBase
    	}
    }

addTxOut()的主要逻辑：

- 1.判断txOut是否是可以花费的。
    - if txscript.IsUnspendable(txOut.PkScript)
- 2.更新utxoentry中的所有字段。
    - 如果entry已经在utxoentry中，则直接更新utxoOutput的各个字段
    - 如果交易输出不在utxoentry中，则创建一个新记录

在connectTransaction()中，通过UtxoEntry的Spend()方法将交易的输入设为已花费，通过UtxoViewpoint的AddTxOuts()方法将交易的
输出添加到utxoset中，可以很明显了解到，主链行区块的变化将直接导致utxoset也跟着变化。当然，区块链的一致性就是主链与utxoset状态的一致性。

其中的关系如下图：


总结一下checkConnectBlock()中验证区块中交易的主要过程：

- 1.根据BIP30的建议，检查区块中是否有重复交易
- 2.将区块中的交易输入引用的utxo从DB中加载到内存
- 3.检查区块中所有交易脚本中（包括P2SH）中操作符的个数是否超过限制
- 4.按区块中交易的顺序，对交易逐个进行双重支付检查，并将交易的输入引用的utxo设为spent，将交易的输出添加到utxoset中，并计算所有交易的费用之和
- 5.检查coinbase的输出是否超过网络预期的奖励与交易费用之和
- 6.如果CSV已经部署，检查各个交易的LockTime是否已经解锁，LockTime小于区块高度或者MTP时间的交易不能被接受
- 7.最后，通过脚本执行引擎运行脚本并检验ECDSA

上面还有LockTime检查相关的calcSequenceLock()和SequenceLockActive()等方法不清楚其意义。
根据[BIP68](https://github.com/bitcoin/bips/blob/master/bip-0068.mediawiki)的定义，TxIn的32bit的Sequence Number可以解释成相对锁定时间，也可以解释成相对锁定高度，编码规则如下：

![](https://github.com/bitcoin/bips/raw/master/bip-0068/encoding.png)

上图中Sequence中的第31位表明相对锁定时间机制未开启
第0-15位表示Sequence的有效值
如果第22位为1则该值表示相对锁定时间，为0表示相对锁定高度
如果第16位表示相对时间，则其精度为512s

    func (b *BlockChain) calcSequenceLock(node *blockNode, tx *btcutil.Tx, utxoView *UtxoViewpoint, mempool bool) (*SequenceLock, error) {
    	// 将Seconds和BlockHeight设置为-1，这样一个相对时间锁定值能允许一个交易能包含任何时间和区块高度。
    	sequenceLock := &SequenceLock{Seconds: -1, BlockHeight: -1}

    	// 对于mempool中的交易，序列锁定语义就是始终处于活动状态
    	csvSoftforkActive := mempool

    	// If we're performing block validation, then we need to query the BIP9 state.
    	if !csvSoftforkActive {
    		// Obtain the latest BIP9 version bits state for the
    		// CSV-package soft-fork deployment. The adherence of sequence
    		// locks depends on the current soft-fork state.
    		csvState, err := b.deploymentState(node.parent, chaincfg.DeploymentCSV)
    		if err != nil {
    			return nil, err
    		}
    		csvSoftforkActive = csvState == ThresholdActive
    	}

    	// If the transaction's version is less than 2, and BIP 68 has not yet
    	// been activated then sequence locks are disabled. Additionally,
    	// sequence locks don't apply to coinbase transactions Therefore, we
    	// return sequence lock values of -1 indicating that this transaction
    	// can be included within a block at any given height or time.
    	mTx := tx.MsgTx()
    	sequenceLockActive := mTx.Version >= 2 && csvSoftforkActive
    	if !sequenceLockActive || IsCoinBase(tx) {
    		return sequenceLock, nil
    	}

    	// Grab the next height from the PoV of the passed blockNode to use for
    	// inputs present in the mempool.
    	nextHeight := node.height + 1

    	for txInIndex, txIn := range mTx.TxIn {
    		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
    		if utxo == nil {
    			str := fmt.Sprintf("output %v referenced from "+
    				"transaction %s:%d either does not exist or "+
    				"has already been spent", txIn.PreviousOutPoint,
    				tx.Hash(), txInIndex)
    			return sequenceLock, ruleError(ErrMissingTxOut, str)
    		}

    		// 如果输入高度设置为mempool高度，那么在评估序列化区块时就要假定交易会打包到下一个区块
    		inputHeight := utxo.BlockHeight()
    		if inputHeight == 0x7fffffff {
    			inputHeight = nextHeight
    		}

    		// 给定一个序列号，我们应用相对时间锁定掩码，以便获得在输入​​可以花费之前所需的时间锁定增量。
    		sequenceNum := txIn.Sequence
    		relativeLock := int64(sequenceNum & wire.SequenceLockTimeMask)

    		switch {
    		// 此输入的相对时间锁定被禁用，因此我们可以跳过任何进一步的计算。
    		case sequenceNum&wire.SequenceLockTimeDisabled == wire.SequenceLockTimeDisabled:
    			continue
    		case sequenceNum&wire.SequenceLockTimeIsSeconds == wire.SequenceLockTimeIsSeconds:
    			// 该输入需要相对时间锁定，以秒表示，然后才能使用。
    			// 因此，我们需要在包含此输入的块之前查询该块，以便可以计算包含此参考输出的块之前的块的过去中值时间。
    			prevInputHeight := inputHeight - 1
    			if prevInputHeight < 0 {
    				prevInputHeight = 0
    			}
    			blockNode := node.Ancestor(prevInputHeight)
    			medianTime := blockNode.CalcPastMedianTime()

    			// 由BIP 68定义的基于时间的相对时间锁具有RelativeLockSeconds的时间粒度，因此我们向左移动该量以转换为适当的相对时间锁。
    			// 我们还从相对锁中减去一个来维护原始的lockTime语义。
    			timeLockSeconds := (relativeLock << wire.SequenceLockTimeGranularity) - 1
    			timeLock := medianTime.Unix() + timeLockSeconds
    			if timeLock > sequenceLock.Seconds {
    				sequenceLock.Seconds = timeLock
    			}
    		default:
    			// 该输入的相对锁定时间以块表示，因此我们计算输入高度的相对偏移作为其转换后的绝对锁定时间。
    			// 我们从相对锁中减去一个以维持原始的lockTime语义。
    			blockHeight := inputHeight + int32(relativeLock-1)
    			if blockHeight > sequenceLock.BlockHeight {
    				sequenceLock.BlockHeight = blockHeight
    			}
    		}
    	}

    	return sequenceLock, nil
    }

其主要逻辑：

- 1.遍历交易的所有交易输入，计算每个交易输入对应的交易所在的区块高度和每个交易输入的Sequence Number里的第16位置。
    - inputHeight := utxo.BlockHeight()
    - sequenceNum := txIn.Sequence
- 2.如果相对锁定时间机制没有开启，就不需要计算该交易输入的相对锁定时间。
    - case sequenceNum&wire.SequenceLockTimeDisabled == wire.SequenceLockTimeDisabled:
- 3.如果Sequence的第22位置为0，则将Sequence值解析成相对时间。
    - case sequenceNum&wire.SequenceLockTimeIsSeconds == wire.SequenceLockTimeIsSeconds:
    - 计算交易输入所在区块的MTP，然后计算相对时间，以秒为单位
    - 计算交易输入对应的绝对解锁时间，然后将交易中所有交易输入的绝对解锁时间的最大值作为交易的解锁时间
- 4.如果Sequence的第22位为1，则将Sequence值解析成相对高度。
    - default:
    - 计算绝对解锁高度
    - 将交易中的所有输入的绝对解锁高度的最大值作为交易的解锁高度

在checkConnectBlock()中，通过calcSequenceLock()计算出区块中交易的解锁时间后，就调用SequenceLockActive()来判断交易是否能被打包进当前区块。

    func SequenceLockActive(sequenceLock *SequenceLock, blockHeight int32,
    	medianTimePast time.Time) bool {

    	// If either the seconds, or height relative-lock time has not yet
    	// reached, then the transaction is not yet mature according to its
    	// sequence locks.
    	if sequenceLock.Seconds >= medianTimePast.Unix() ||
    		sequenceLock.BlockHeight >= blockHeight {
    		return false
    	}

    	return true
    }

SequenceLockActive确定交易的序列锁是否已经被满足，这意味着给定交易的所有输入已达到足够的高度或时间，以满足其相对锁定时间成熟度。
只有当交易的绝对解锁时间和绝对解锁高度均小于当前区块的MTP和高度时，交易的sequence lock才算解锁。
通俗理解，交易输入花费的所有交易均满足一定成熟度要求。
当然，如果SequenceLock中的高度和时间均为-1，表明交易可以被打包进任何区块中。
如何理解交易输入的锁定时间？
如果LockTime值为10000，则交易只能被打包进10001及以后的区块中。
如果该交易的输入交易所在区块高度为9999，且Sequence指定的相对高度为100，则该交易只能被打包进10098及以后的区块中。

checkBlockContext()中会用IsFinalizedTransaction()对交易的LockTime进行检查，在checkConnectBlock()中如果BIP68部署了，
就通过SequenceLockActive()对交易输入的Seqence表示的相对锁定时间或相对锁定高度进行检查，这是为了兼容两种锁定时间。

checkConnectBlock()中的各项检查通过后，节点就会调用connectBlock()将区块相关状态写入数据库。

    func (b *BlockChain) connectBlock(node *blockNode, block *btcutil.Block, view *UtxoViewpoint, stxos []spentTxOut) error {
    	// Make sure it's extending the end of the best chain.
    	prevHash := &block.MsgBlock().Header.PrevBlock
    	if !prevHash.IsEqual(&b.bestChain.Tip().hash) {
    		return AssertError("connectBlock must be called with a block " +
    			"that extends the main chain")
    	}

    	// Sanity check the correct number of stxos are provided.
    	if len(stxos) != countSpentOutputs(block) {
    		return AssertError("connectBlock called with inconsistent " +
    			"spent transaction out information")
    	}

    	// No warnings about unknown rules or versions until the chain is current.
    	if b.isCurrent() {
    		// Warn if any unknown new rules are either about to activate or have already been activated.
    		if err := b.warnUnknownRuleActivations(node); err != nil {
    			return err
    		}

    		// Warn if a high enough percentage of the last blocks have unexpected versions.
    		if err := b.warnUnknownVersions(node); err != nil {
    			return err
    		}
    	}

    	// Write any block status changes to DB before updating best state.
    	err := b.index.flushToDB()
    	if err != nil {
    		return err
    	}

    	// Generate a new best state snapshot that will be used to update the
    	// database and later memory if all database updates are successful.
    	b.stateLock.RLock()
    	curTotalTxns := b.stateSnapshot.TotalTxns
    	b.stateLock.RUnlock()
    	numTxns := uint64(len(block.MsgBlock().Transactions))
    	blockSize := uint64(block.MsgBlock().SerializeSize())
    	blockWeight := uint64(GetBlockWeight(block))
    	state := newBestState(node, blockSize, blockWeight, numTxns,
    		curTotalTxns+numTxns, node.CalcPastMedianTime())

    	// Atomically insert info into the database.
    	err = b.db.Update(func(dbTx database.Tx) error {
    		// Update best block state.
    		err := dbPutBestState(dbTx, state, node.workSum)
    		if err != nil {
    			return err
    		}

    		// Add the block hash and height to the block index which tracks the main chain.
    		err = dbPutBlockIndex(dbTx, block.Hash(), node.height)
    		if err != nil {
    			return err
    		}

    		// Update the utxo set using the state of the utxo view.  This
    		// entails removing all of the utxos spent and adding the new
    		// ones created by the block.
    		err = dbPutUtxoView(dbTx, view)
    		if err != nil {
    			return err
    		}

    		// Update the transaction spend journal by adding a record for
    		// the block that contains all txos spent by it.
    		err = dbPutSpendJournalEntry(dbTx, block.Hash(), stxos)
    		if err != nil {
    			return err
    		}

    		// Allow the index manager to call each of the currently active
    		// optional indexes with the block being connected so they can
    		// update themselves accordingly.
    		if b.indexManager != nil {
    			err := b.indexManager.ConnectBlock(dbTx, block, view)
    			if err != nil {
    				return err
    			}
    		}

    		return nil
    	})
    	if err != nil {
    		return err
    	}

    	// Prune fully spent entries and mark all entries in the view unmodified
    	// now that the modifications have been committed to the database.
    	view.commit()

    	// This node is now the end of the best chain.
    	b.bestChain.SetTip(node)

    	// Update the state for the best block.  Notice how this replaces the
    	// entire struct instead of updating the existing one.  This effectively
    	// allows the old version to act as a snapshot which callers can use
    	// freely without needing to hold a lock for the duration.  See the
    	// comments on the state variable for more details.
    	b.stateLock.Lock()
    	b.stateSnapshot = state
    	b.stateLock.Unlock()

    	// Notify the caller that the block was connected to the main chain.
    	// The caller would typically want to react with actions such as
    	// updating wallets.
    	b.chainLock.Unlock()
    	b.sendNotification(NTBlockConnected, block)
    	b.chainLock.Lock()

    	return nil
    }

调用connectBestChain()将区块连入区块链之前，区块本身就已经写入区块文件了，connectBlock()只是将区块链的最新状态更新到数据库中。

- 1.两个检查，一个是保证父区块在主链上是tip节点，另外一个是保证区块中花费的交易数量与spentTxOuts数量是一致的。
    - if !prevHash.IsEqual(&b.bestChain.Tip().hash) {
    - if len(stxos) != countSpentOutputs(block) {
- 2.有未知的软分叉部署在区块中激活（状态为ThresholdActive或ThresholdLockedIn），打印告警log。
    - 统计区块前100个区块中有这种情况的区块个数，超过50%时，打印告警log提醒用户升级节点版本
    - if err := b.warnUnknownVersions(node); err != nil {
- 3. 在更新best state之前，先降任何的区块状态更改写入到数据库。
    - err := b.index.flushToDB()
- 4.新区块的Hash，高度，难度Bits，交易数量，MTP及主链上总的交易数量构造新的主链BsetState。
    - state := newBestState(node, blockSize, blockWeight, numTxns,curTotalTxns+numTxns, node.CalcPastMedianTime())
- 5.开始更新数据库，先将主链的新的BestState，然后总的工作量之和更新到键值chainstate中。
    - err := dbPutBestState(dbTx, state, node.workSum)
- 6.将区块hash和区块高度添加到区块索引中，这个区块索引用来跟踪主链的。
    - err = dbPutBlockIndex(dbTx, block.Hash(), node.height)
- 7.删除已经被花费的utxoentry，增加或更新新的utxoentry。
    - err = dbPutUtxoView(dbTx, view)
- 8.通过为区块添加一条记录，这条记录包含所有区块花费和区块hash之间的对应关系，这样来更新交易花费journal
    - err = dbPutSpendJournalEntry(dbTx, block.Hash(), stxos)
- 9.调用indexManager的ConnectBlock函数来更新Indexers中的记录。
    - err := b.indexManager.ConnectBlock(dbTx, block, view)
- 10.由于修改已提交到数据库，因此修剪完全花费的条目并在视图中标记未修改的所有条目。
    - view.commit()
- 11.设置此节点现在是最佳链的末端，将新的区块添加到BlockChain的索引器中，便于后续查找。
    - bestChain.SetTip(node)
- 12.更新stateSnapshot，将主链的快照更新。
    - b.stateSnapshot = state
- 13.向外发出NTBlockConnected事件通知，mempool将更新交易池中的交易，矿工将停止当前挖矿过程开始挖下一个区块。
    - b.sendNotification(NTBlockConnected, block)

通过分析connectBlock()，除了区块本身需要持久化外，与区块链相关的状态也以MetaData的形式存入数据库。

新的区块如果不是扩展主链而是侧链，而扩展侧链之后的总工作量之和大于主链的，那需要通过reorganizeChain()来将主侧地位对调。
同时，区块被添加到主链或者由于reorganizeChain()区块从主链上移除，需要进一步处理孤儿池中的孤儿区块。

如果需要进行主侧链对调，区块需要从主链上移除时，需要做两件事情：

- 通过spendjournal记录的区块中交易花费的utxos将重新回到utxoset中，交易输出的utxo则从utxoset中删除。
- 区块加入主链或者侧链后，如果有孤儿区块的父区块是该区块，则还要将孤儿区块添加到区块链上，并从孤儿池中移出，这个过程是递归操作的。

在分析connectBestChain()时，如果区块的父区块不是主链上的尾节点，则这个区块将被添加到侧链上。
如果侧链的工作量之和超过了主链的工作量，则侧链要通过reorganizeChain()变成主链。
在调用reorganizeChain()之前，要通过getReorganizeNodes()方法找到主链上分叉之后的所有区块和侧链上所有区块。


我们正在扩展（或创建）一个侧链，这个新的侧链的累积工作量总和超过了旧的最佳链，所以这个侧链需要成为主链。
为了实现这一点，找到fork的两侧的共同祖先，断开形成（现在）旧分叉的块与主链的连接，并将形成新链的块连接到从主链开始的主链ancenstor（链分叉的点）。

    func (b *BlockChain) getReorganizeNodes(node *blockNode) (*list.List, *list.List) {
    	attachNodes := list.New()
    	detachNodes := list.New()

    	// Do not reorganize to a known invalid chain. Ancestors deeper than the
    	// direct parent are checked below but this is a quick check before doing
    	// more unnecessary work.
    	if b.index.NodeStatus(node.parent).KnownInvalid() {
    		b.index.SetStatusFlags(node, statusInvalidAncestor)
    		return detachNodes, attachNodes
    	}

    	// Find the fork point (if any) adding each block to the list of nodes
    	// to attach to the main tree.  Push them onto the list in reverse order
    	// so they are attached in the appropriate order when iterating the list later.
    	forkNode := b.bestChain.FindFork(node)
    	invalidChain := false
    	for n := node; n != nil && n != forkNode; n = n.parent {
    		if b.index.NodeStatus(n).KnownInvalid() {
    			invalidChain = true
    			break
    		}
    		attachNodes.PushFront(n)
    	}

    	// If any of the node's ancestors are invalid, unwind attachNodes, marking
    	// each one as invalid for future reference.
    	if invalidChain {
    		var next *list.Element
    		for e := attachNodes.Front(); e != nil; e = next {
    			next = e.Next()
    			n := attachNodes.Remove(e).(*blockNode)
    			b.index.SetStatusFlags(n, statusInvalidAncestor)
    		}
    		return detachNodes, attachNodes
    	}

    	// Start from the end of the main chain and work backwards until the
    	// common ancestor adding each block to the list of nodes to detach from
    	// the main chain.
    	for n := b.bestChain.Tip(); n != nil && n != forkNode; n = n.parent {
    		detachNodes.PushBack(n)
    	}

    	return detachNodes, attachNodes
    }

getReorganizeNodes()方法一开始进行快速检查，如果是一个已知的无效的链，那就不需要进行reorganize。
至少Ancestors要比当前的父节点要更加深。
从侧链的链尾（刚加入的区块）向前遍历，知道主链与侧链的分叉点，并将侧链上的所有区块节点按原来的顺序添加到attachNodes中，
这样保证这些区块按其先后顺序依次加入主链。
如果这个节点的任何祖先是无效的，展开attachNodes，然后将他们每个都标记为无效以供将来引用。
接着从主链的链尾向前遍历知道分叉点，并将访问到的区块节点按相反的书序添加到detachNodes中，这样链尾节点是detachNodes的第一个元素，
这样方便从链尾到分叉点依次从主链上移除。

getReorganizeNodes()收集完待移除和待添加的区块节点后，由reorganizeChain()完成侧链变成主链的过程。

    func (b *BlockChain) reorganizeChain(detachNodes, attachNodes *list.List) error {
    	// 所有将要移除的区块，与之相关的区块里，spend journal实例需要unspend转换成outputs，这些区块都是
    	// 没有连接的，都需要从数据库中加载，他们都是需要进行检查和操作多步骤，可以进行缓存到slices中。
    	detachBlocks := make([]*btcutil.Block, 0, detachNodes.Len())
    	detachSpentTxOuts := make([][]spentTxOut, 0, detachNodes.Len())
    	attachBlocks := make([]*btcutil.Block, 0, attachNodes.Len())

    	// 断开所有区块到分叉点的连接。这需要从数据库加载区块以及相关的已使用的txos，
    	// 并使用该信息来取消所有已用完的txos并删除由区块创建的utxos。
    	view := NewUtxoViewpoint()
    	view.SetBestHash(&b.bestChain.Tip().hash)
    	for e := detachNodes.Front(); e != nil; e = e.Next() {
    		n := e.Value.(*blockNode)
    		var block *btcutil.Block
    		err := b.db.View(func(dbTx database.Tx) error {
    			var err error
    			block, err = dbFetchBlockByNode(dbTx, n)
    			return err
    		})
    		if err != nil {
    			return err
    		}

    		// Load all of the utxos referenced by the block that aren't already in the view.
    		err = view.fetchInputUtxos(b.db, block)
    		if err != nil {
    			return err
    		}

    		// Load all of the spent txos for the block from the spend journal.
    		var stxos []spentTxOut
    		err = b.db.View(func(dbTx database.Tx) error {
    			stxos, err = dbFetchSpendJournalEntry(dbTx, block)
    			return err
    		})
    		if err != nil {
    			return err
    		}

    		// Store the loaded block and spend journal entry for later.
    		detachBlocks = append(detachBlocks, block)
    		detachSpentTxOuts = append(detachSpentTxOuts, stxos)

    		err = view.disconnectTransactions(b.db, block, stxos)
    		if err != nil {
    			return err
    		}
    	}

    	// 执行多项检查，用来验证需要连接到主链的每个区块可以连接而不违反任何规则并且不实际连接区块。
    	//
    	// NOTE: These checks could be done directly when connecting a block,
    	// however the downside to that approach is that if any of these checks
    	// fail after disconnecting some blocks or attaching others, all of the
    	// operations have to be rolled back to get the chain back into the
    	// state it was before the rule violation (or other failure).  There are
    	// at least a couple of ways accomplish that rollback, but both involve
    	// tweaking the chain and/or database.  This approach catches these
    	// issues before ever modifying the chain.
    	var validationError error
    	for e := attachNodes.Front(); e != nil; e = e.Next() {
    		n := e.Value.(*blockNode)

    		// 如果attachNodes中的任何先前节点验证失败，请将此标记标记为具有无效的祖先。
    		if validationError != nil {
    			b.index.SetStatusFlags(n, statusInvalidAncestor)
    			continue
    		}

    		var block *btcutil.Block
    		err := b.db.View(func(dbTx database.Tx) error {
    			var err error
    			block, err = dbFetchBlockByNode(dbTx, n)
    			return err
    		})
    		if err != nil {
    			return err
    		}

    		// Store the loaded block for later.
    		attachBlocks = append(attachBlocks, block)

    		// 如果节点已经被完全验证过了，就跳过检查。虽然跳过了checkConnectBlock，但我们仍然需要更新UTXO视图。
    		if b.index.NodeStatus(n).KnownValid() {
    			err = view.fetchInputUtxos(b.db, block)
    			if err != nil {
    				return err
    			}
    			err = view.connectTransactions(block, nil)
    			if err != nil {
    				return err
    			}
    			continue
    		}

    		// spent txout细节在这里是没有被请求的，因此也不会生成。
    		// 这样做是因为这些状态没有立即写入数据库，因此不需要。
    		err = b.checkConnectBlock(n, block, view, nil)
    		if err != nil {
    			// 如果区块验证失败，则将其标记为无效，然后继续循环其余节点，将其标记为具有无效的祖先
    			if _, ok := err.(RuleError); ok {
    				b.index.SetStatusFlags(n, statusValidateFailed)
    				validationError = err
    				continue
    			}
    			return err
    		}
    		b.index.SetStatusFlags(n, statusValid)
    	}

    	if validationError != nil {
    		return validationError
    	}

    	// 重置以下实际连接代码的视图。
    	// 这是必须的，因为先前在检查重组是否成功时修改了视图
    	// 并且连接代码的视图要求视图从每个区块连接或断开的角度来看是否有效的。
    	view = NewUtxoViewpoint()
    	view.SetBestHash(&b.bestChain.Tip().hash)

    	// Disconnect blocks from the main chain.
    	for i, e := 0, detachNodes.Front(); e != nil; i, e = i+1, e.Next() {
    		n := e.Value.(*blockNode)
    		block := detachBlocks[i]

    		// 加载被区块所引用的未在视图中的所有utxos
    		err := view.fetchInputUtxos(b.db, block)
    		if err != nil {
    			return err
    		}

    		// 更新视图，unspend所有已花费的txos，并删除由该区块创建的utxos
    		err = view.disconnectTransactions(b.db, block,
    			detachSpentTxOuts[i])
    		if err != nil {
    			return err
    		}

    		// Update the database and chain state.
    		err = b.disconnectBlock(n, block, view)
    		if err != nil {
    			return err
    		}
    	}

    	// Connect the new best chain blocks.
    	for i, e := 0, attachNodes.Front(); e != nil; i, e = i+1, e.Next() {
    		n := e.Value.(*blockNode)
    		block := attachBlocks[i]

    		// 加载区块中引用的所有未在视图中的utxos
    		err := view.fetchInputUtxos(b.db, block)
    		if err != nil {
    			return err
    		}

    		// 更新视图，将区块中所有引用的utxos标记为已花费
    		// 并将由此区块创建的所有交易添加到该视图中
    		// 此外，提供stxo切片以生成已花费的txout详细信息
    		stxos := make([]spentTxOut, 0, countSpentOutputs(block))
    		err = view.connectTransactions(block, &stxos)
    		if err != nil {
    			return err
    		}

    		// Update the database and chain state.
    		err = b.connectBlock(n, block, view, stxos)
    		if err != nil {
    			return err
    		}
    	}

    	// Log the point where the chain forked and old and new best chain heads.
    	firstAttachNode := attachNodes.Front().Value.(*blockNode)
    	firstDetachNode := detachNodes.Front().Value.(*blockNode)
    	lastAttachNode := attachNodes.Back().Value.(*blockNode)
    	log.Infof("REORGANIZE: Chain forks at %v", firstAttachNode.parent.hash)
    	log.Infof("REORGANIZE: Old best chain head was %v", firstDetachNode.hash)
    	log.Infof("REORGANIZE: New best chain head is %v", lastAttachNode.hash)

    	return nil
    }

reorganizeChain()在进行主链切换时分为两个阶段：

- 对切换过程中涉及到的block，transaction及utxoset操作进行验证。
- 进行真正的移除和添加操作。

这两个阶段是分开的，因为如果是变验证边移除添加操作，在中途遇到验证失败的情况，需要进行回滚操作。
同时这两个阶段使用了不同的UtxoViewpoint，涉及到对utxoset的操作互不影响。

之前getReorganizeNodes()对需要添加和移除的区块进行了缓存，在验证和操作阶段，可能要访问待移除和添加的区块信息
和待移除区块的spendJournal，可能需要从数据库中读取。
在主侧链进行切换的时候，先从主链移除区块，再添加侧链上的区块。

reorganizeChain()的主要步骤：

- 1.先定义并初始化缓存，detachBlocks是主链上需要移除的区块，attachBlocks是将要添加的侧链上的区块，detachSpentTxOuts是将要移除区块的spentTxOut集合。
- 2.从数据库中加载将要移除区块及它花费的utxoentry，spentTxouts，并缓存下来。
- 3.新建UtxoViewpoint，对utxoset操作，包括将交易产生的utxo清除和交易花费的txout重新放回utxoset，最后disconnectTransactions()进行关闭。
- 4.首先将待移除的区块从链尾向前逐个从主链上移除，在新的UtxoViewpoint中操作。
- 5.调用disconnectBlock()将区块从主链移除。
    - err = b.disconnectBlock(n, block, view)
- 6.区块移除完成后，接着将原侧链上的区块逐个添加到主链。需要从数据库中将区块花费的utxo加载到UtxoViewpoint中。
- 7.调用UtxoViewpoint的connectTransactions()将区块花费的utxo设为已花费并记录区块花费的所有spentTxOut。
- 8.最后调用connectBlock()将区块写入主链。

上面的reorganizeChain()方法中有调用disconnectTransactions()，这个方法和connectTransaction()是相反的功能。

    func (view *UtxoViewpoint) disconnectTransactions(db database.DB, block *btcutil.Block, stxos []spentTxOut) error {
    	// Sanity check the correct number of stxos are provided.
    	if len(stxos) != countSpentOutputs(block) {
    		return AssertError("disconnectTransactions called with bad " +
    			"spent transaction out information")
    	}

    	// 循环遍历所有交易，因为stxos是unspent的，也是以相反的顺序排列。
    	// 这是必须的，因为区块中后面的交易可以从前面的交易中花费。
    	stxoIdx := len(stxos) - 1
    	transactions := block.Transactions()
    	for txIdx := len(transactions) - 1; txIdx > -1; txIdx-- {
    		tx := transactions[txIdx]

    		// All entries will need to potentially be marked as a coinbase.
    		var packedFlags txoFlags
    		isCoinBase := txIdx == 0
    		if isCoinBase {
    			packedFlags |= tfCoinBase
    		}

    		// 将交易最初创建的可花费的outputs标记为已花费。
    		// 值得指出的是，虽然outputs实际上并没有在这里使用，而是它们已经不再存在，
    		// 因为使用了修剪后的utxo集，所以不存在的utxo和已经花费的utxo之间没有实际区别。
    		// 如果视图中尚不存在utxo，请为其添加条目，然后将其标记为已花费。
    		// 这样做的目的是因为代码依赖它在视图中的存在，以便发生信号修改。
    		txHash := tx.Hash()
    		prevOut := wire.OutPoint{Hash: *txHash}
    		for txOutIdx, txOut := range tx.MsgTx().TxOut {
    			if txscript.IsUnspendable(txOut.PkScript) {
    				continue
    			}

    			prevOut.Index = uint32(txOutIdx)
    			entry := view.entries[prevOut]
    			if entry == nil {
    				entry = &UtxoEntry{
    					amount:      txOut.Value,
    					pkScript:    txOut.PkScript,
    					blockHeight: block.Height(),
    					packedFlags: packedFlags,
    				}

    				view.entries[prevOut] = entry
    			}

    			entry.Spend()
    		}

    		// 循环遍历所有交易输入（没有输入的coinbase除外）并取消引用的txos，这是匹配已花费的txout条目的顺序所必须的。
    		if isCoinBase {
    			continue
    		}
    		for txInIdx := len(tx.MsgTx().TxIn) - 1; txInIdx > -1; txInIdx-- {
    			// 确保已花费的txout索引递减以与交易输入保持同步
    			stxo := &stxos[stxoIdx]
    			stxoIdx--

    			// 如果视图中没有引用output的条目，则表示它先前已经花费完，因此创建一个新的utxo条目以便重新生成它
    			originOut := &tx.MsgTx().TxIn[txInIdx].PreviousOutPoint
    			entry := view.entries[*originOut]
    			if entry == nil {
    				entry = new(UtxoEntry)
    				view.entries[*originOut] = entry
    			}

    			// The legacy v1 spend journal format only stored the
    			// coinbase flag and height when the output was the last
    			// unspent output of the transaction.  As a result, when
    			// the information is missing, search for it by scanning
    			// all possible outputs of the transaction since it must
    			// be in one of them.
    			//
    			// It should be noted that this is quite inefficient,
    			// but it realistically will almost never run since all
    			// new entries include the information for all outputs
    			// and thus the only way this will be hit is if a long
    			// enough reorg happens such that a block with the old
    			// spend data is being disconnected.  The probability of
    			// that in practice is extremely low to begin with and
    			// becomes vanishingly small the more new blocks are
    			// connected.  In the case of a fresh database that has
    			// only ever run with the new v2 format, this code path
    			// will never run.
    			if stxo.height == 0 {
    				utxo, err := view.fetchEntryByHash(db, txHash)
    				if err != nil {
    					return err
    				}
    				if utxo == nil {
    					return AssertError(fmt.Sprintf("unable "+
    						"to resurrect legacy stxo %v",
    						*originOut))
    				}

    				stxo.height = utxo.BlockHeight()
    				stxo.isCoinBase = utxo.IsCoinBase()
    			}

    			// 使用花费journal中的stxo数据恢复utxo并将其标记为已修改
    			entry.amount = stxo.amount
    			entry.pkScript = stxo.pkScript
    			entry.blockHeight = stxo.height
    			entry.packedFlags = tfModified
    			if stxo.isCoinBase {
    				entry.packedFlags |= tfCoinBase
    			}
    		}
    	}

    	// 当现在区块中所有的交易都断开连接，就需要为之前的区块的视图更新最佳hash
    	view.SetBestHash(&block.MsgBlock().Header.PrevBlock)
    	return nil
    }

disconnectTransactions通过删除传递的区块创建的所有交易来更新视图。通过使用提供的已花费的txo信息不想恢复所花费的交易的所有utxos。
在传递区块之前为视图的区块最佳hash进行设置。
disconnectTransactions()核心功能是将区块中的交易从utxoset中移除，并将交易已经花费的所有spentTxOut重设为unspent。

其主要逻辑：

- 1.对区块花费的交易数量做检查，确保从数据库中读取到的spent journal与待处理的区块是对应的。
    - if len(stxos) != countSpentOutputs(block) {
- 2.从最后一个交易开始往前遍历处理区块中的交易
    - 从最后一个开始处理的逻辑是后面的交易可能花费前面的交易
    - 而处理过程中，要把交易本身从utxoset中移除，同时将已经花费的交易恢复到utxoset中
    - 交易将按照倒序遍历，处理交易中的输入时也是按照倒序处理
- 3. 循环遍历所有交易输入（没有输入的coinbase除外）并取消引用的txos，这是匹配已花费的txout条目的顺序所必须的。
    - utxo, err := view.fetchEntryByHash(db, txHash)
    - 调用fetchEntryByHash()将区块中的utxo从数据库中加载出来
    - 确保已花费的txout索引递减以与交易输入保持同步
    - 如果视图中没有引用output的条目，则表示它先前已经花费完，因此创建一个新的utxo条目以便重新生成它
- 4.根据spentTxout恢复utxoentry中的每一项输出，并将其输出的状态设为unspent。
- 5.处理完区块中所有交易后，将UtxoViewpoint的观察点移向父区块。
    - view.SetBestHash(&block.MsgBlock().Header.PrevBlock)

connectBlock()是将区块最终写入主链，那么要将区块从主链移除就是disconnectBlock()：

    func (b *BlockChain) disconnectBlock(node *blockNode, block *btcutil.Block, view *UtxoViewpoint) error {
    	// Make sure the node being disconnected is the end of the best chain.
    	if !node.hash.IsEqual(&b.bestChain.Tip().hash) {
    		return AssertError("disconnectBlock must be called with the " +
    			"block at the end of the main chain")
    	}

    	// 加载前一个区块，因为下面需要它的一些细节
    	prevNode := node.parent
    	var prevBlock *btcutil.Block
    	err := b.db.View(func(dbTx database.Tx) error {
    		var err error
    		prevBlock, err = dbFetchBlockByNode(dbTx, prevNode)
    		return err
    	})
    	if err != nil {
    		return err
    	}

    	// 在更新最佳状态之前，将任何区块状态更新写入DB
    	err = b.index.flushToDB()
    	if err != nil {
    		return err
    	}

    	// 生成新的最佳状态快照，如果所有数据库更新都成功，将用于更新数据库和之后的内存。
    	b.stateLock.RLock()
    	curTotalTxns := b.stateSnapshot.TotalTxns
    	b.stateLock.RUnlock()
    	numTxns := uint64(len(prevBlock.MsgBlock().Transactions))
    	blockSize := uint64(prevBlock.MsgBlock().SerializeSize())
    	blockWeight := uint64(GetBlockWeight(prevBlock))
    	newTotalTxns := curTotalTxns - uint64(len(block.MsgBlock().Transactions))
    	state := newBestState(prevNode, blockSize, blockWeight, numTxns,
    		newTotalTxns, prevNode.CalcPastMedianTime())

    	err = b.db.Update(func(dbTx database.Tx) error {
    		// Update best block state.
    		err := dbPutBestState(dbTx, state, node.workSum)
    		if err != nil {
    			return err
    		}

    		// 从跟踪主链的区块索引中删除区块hash和高度
    		err = dbRemoveBlockIndex(dbTx, block.Hash(), node.height)
    		if err != nil {
    			return err
    		}

    		// 使用utxo视图的状态更新utxo集合。这需要恢复花费的所有utxos并删除区块创建的新utxos。
    		err = dbPutUtxoView(dbTx, view)
    		if err != nil {
    			return err
    		}

    		// 通过删除包含区块所花费的所有txos的记录来更新交易支出日志
    		err = dbRemoveSpendJournalEntry(dbTx, block.Hash())
    		if err != nil {
    			return err
    		}

    		// 允许索引管理器在块被断开的情况下调用每个当前活动的可选索引，以便它们可以相应地更新自己。
    		if b.indexManager != nil {
    			err := b.indexManager.DisconnectBlock(dbTx, block, view)
    			if err != nil {
    				return err
    			}
    		}

    		return nil
    	})
    	if err != nil {
    		return err
    	}

    	// Prune fully spent entries and mark all entries in the view unmodified
    	// now that the modifications have been committed to the database.
    	view.commit()

    	// This node's parent is now the end of the best chain.
    	b.bestChain.SetTip(node.parent)

    	// Update the state for the best block.  Notice how this replaces the
    	// entire struct instead of updating the existing one.  This effectively
    	// allows the old version to act as a snapshot which callers can use
    	// freely without needing to hold a lock for the duration.  See the
    	// comments on the state variable for more details.
    	b.stateLock.Lock()
    	b.stateSnapshot = state
    	b.stateLock.Unlock()

    	// Notify the caller that the block was disconnected from the main
    	// chain.  The caller would typically want to react with actions such as
    	// updating wallets.
    	b.chainLock.Unlock()
    	b.sendNotification(NTBlockDisconnected, block)
    	b.chainLock.Lock()

    	return nil
    }

disconnectBlock()的主要逻辑：

- 1.比较待移除区块的Hash是否与链尾节点的Hash值相等，保证移除的是主链链尾节点。
- 2.通过dbFetchBlockByNode加载前一个区块，因为下面需要它的一些细节。
- 3.在更新最佳状态之前，将任何区块状态更新写入DB
- 4.通过前一个区块的细节生成新的最佳状态快照，如果所有数据库更新都成功，将用于更新数据库和之后的内存。
- 5.更新最佳区块状态，从跟踪主链的区块索引中删除区块hash和高度
- 6.dbPutUtxoView()使用utxo视图的状态更新utxo集合。这需要恢复花费的所有utxos并删除区块创建的新utxos。
- 7.dbRemoveSpendJournalEntry()通过删除包含区块所花费的所有txos的记录来更新交易支出日志
- 8.允许索引管理器在块被断开的情况下调用每个当前活动的可选索引，以便它们可以相应地更新自己。
- 9.提交视图更新，更新区块链尾节点，更新主链最佳状态快照。
- 10.向blockManager通知NTBlockDisconnected事件，将区块中的交易重新放回mempool。

以上都分析完了侧链切换成主链的整个流程，其中比较关键的点有：

- 伴随区块移除或者添加到主链上时，数据库和内存中utxoset的变化，其中还有大量的验证过程。
- 区块扩展主链或者扩展侧链，节点要检查孤儿区块池中是否有孤儿的父区块就是刚刚连入区块链的区块。
- 如果要对孤儿区块进行处理，那就是processOrphans()中实现。
- 当然孤儿区块是怎么来的呢，很简单，processBlock()的时候，准备连入区块链的区块压根就找不到父区块，则调用addOrphanBlock()将其添加到孤儿区块池中。

    func (b *BlockChain) addOrphanBlock(block *btcutil.Block) {
    	// Remove expired orphan blocks.
    	for _, oBlock := range b.orphans {
    		if time.Now().After(oBlock.expiration) {
    			b.removeOrphanBlock(oBlock)
    			continue
    		}

    		// Update the oldest orphan block pointer so it can be discarded
    		// in case the orphan pool fills up.
    		if b.oldestOrphan == nil || oBlock.expiration.Before(b.oldestOrphan.expiration) {
    			b.oldestOrphan = oBlock
    		}
    	}

    	// Limit orphan blocks to prevent memory exhaustion.
    	if len(b.orphans)+1 > maxOrphanBlocks {
    		// Remove the oldest orphan to make room for the new one.
    		b.removeOrphanBlock(b.oldestOrphan)
    		b.oldestOrphan = nil
    	}

    	// Protect concurrent access.  This is intentionally done here instead
    	// of near the top since removeOrphanBlock does its own locking and
    	// the range iterator is not invalidated by removing map entries.
    	b.orphanLock.Lock()
    	defer b.orphanLock.Unlock()

    	// Insert the block into the orphan map with an expiration time
    	// 1 hour from now.
    	expiration := time.Now().Add(time.Hour)
    	oBlock := &orphanBlock{
    		block:      block,
    		expiration: expiration,
    	}
    	b.orphans[*block.Hash()] = oBlock

    	// Add to previous hash lookup index for faster dependency lookups.
    	prevHash := &block.MsgBlock().Header.PrevBlock
    	b.prevOrphans[*prevHash] = append(b.prevOrphans[*prevHash], oBlock)
    }

其主要逻辑：

- 1.遍历孤儿池，将其中超过生存期的孤儿区块从中移除，孤儿区块的生存期为1个小时，同时更新年龄最大的孤儿区块。
- 2.为了保证内存的较小消耗，生存期审核过后，如果孤儿区块数量还大于99，则最老的孤儿区块移除，保证孤儿池中孤儿区块不超过100个。
- 3.在清理完孤儿池后，封装orphanBlock对象，将生存期设置为1个小时。
- 4.将孤儿区块添加到孤儿池中。
- 5.将孤儿区块添加到b.prevOrphans中，b.prevOrphans以孤儿区块父区块Hash为key记录了同一父区块的所有孤儿区块。

添加孤儿区块分析完，看看处理孤儿区块的方法processOrphans()：

    func (b *BlockChain) processOrphans(hash *chainhash.Hash, flags BehaviorFlags) error {
    	// 处理传递的hash开始，为需要处理的其他孤儿区块流出一点空间，而不需要在常见情况下扩展array
    	processHashes := make([]*chainhash.Hash, 0, 10)
    	processHashes = append(processHashes, hash)
    	for len(processHashes) > 0 {
    		// Pop the first hash to process from the slice.
    		processHash := processHashes[0]
    		processHashes[0] = nil // Prevent GC leak.
    		processHashes = processHashes[1:]

    		// Look up all orphans that are parented by the block we just
    		// accepted.  This will typically only be one, but it could
    		// be multiple if multiple blocks are mined and broadcast
    		// around the same time.  The one with the most proof of work
    		// will eventually win out.  An indexing for loop is
    		// intentionally used over a range here as range does not
    		// reevaluate the slice on each iteration nor does it adjust the
    		// index for the modified slice.
    		for i := 0; i < len(b.prevOrphans[*processHash]); i++ {
    			orphan := b.prevOrphans[*processHash][i]
    			if orphan == nil {
    				log.Warnf("Found a nil entry at index %d in the "+
    					"orphan dependency list for block %v", i,
    					processHash)
    				continue
    			}

    			// Remove the orphan from the orphan pool.
    			orphanHash := orphan.block.Hash()
    			b.removeOrphanBlock(orphan)
    			i--

    			// Potentially accept the block into the block chain.
    			_, err := b.maybeAcceptBlock(orphan.block, flags)
    			if err != nil {
    				return err
    			}

    			// Add this block to the list of blocks to process so
    			// any orphan blocks that depend on this block are
    			// handled too.
    			processHashes = append(processHashes, orphanHash)
    		}
    	}
    	return nil
    }

processOrphans()的输入参数hash是刚添加到区块链上的区块的Hash，而不是孤儿区块的Hash：

- 1.申明一个临时的slice来处理processHashes，用于缓存刚添加到区块链上的区块Hash。
- 2.循环处理所有刚添加到区块链上的区块为父区块的孤儿。
    - 从processHashes中取出首元素，并将首元素从processHashes中移除，循环遍历processHashes中的元素
- 3.父区块有孤儿区块，则循环处理其他所有孤儿
- 4.处理孤儿区块时，首先将其从孤儿池中移除，调用maybeAcceptBlock()将其加入到区块链上。
- 5.如果孤儿区块正确加入区块链，则将其添加到processHashes中，继续检查是否还有以它为父区块的孤儿。
    - 如果有，则重复上面的步骤，直到新添加到区块链中的区块没有孤儿子区块在孤儿池中为止。

processBlock()总体是三个大的步骤，第一步是调用checkBlockSanity()对区块进行完整性检查，第二步是调用
maybeAcceptBlock()将区块添加到区块链上；第三步是调用processOrphans()对孤儿区块进行处理。

