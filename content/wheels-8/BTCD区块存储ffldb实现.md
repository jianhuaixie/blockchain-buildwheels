# BTCD区块存储ffldb实现

在[database](https://github.com/btcsuite/btcd/tree/master/database)这个目录,是BTCD的存储框架，能使得btcd支持多种数据库。
其中ffldb是database包中提供的默认数据库。

database目录主要包含的文件有:

- cmd/dbtool :实现了一个从db文件中读写block的工具
- ffldb :实现了一个默认的数据库驱动，参考了BoltDB实现了DB,Bucket,Tx等
- internal/treap :一个[数堆](https://zh.wikipedia.org/zh-cn/%E6%A0%91%E5%A0%86)的实现，用于缓存元数据
- testdata :包含用于测试的db文件
- driver.go :定义了Driver类型及注册，打开数据库的方法
- interface.go :定义了DB，Bucket，Tx，Cursor等接口，几乎和BoltDB中的定义一致
- error.go :定义了包database中的错误代码及对应的提示字符


### cmd/dbtool

这个目录下面有7个文件

- main.go
    - func loadBlockDB() (database.DB,error):导入数据库
    - func realMain() error:设置解析的参数和命令，然后启动命令执行，其中包括四个命令：insecureimport，loadheaders，fetchblock，fetchblockregion
    - func main():设置CPU核使用数量，入口方法
- globalconfig.go
    - type config struct: 定义全局配置选项，有DataDir，DbType，TestNet3，RegressionTest，SimNet，第一个是btcd数据存放目录，第二个是数据库后端选用类型，后三个是network
    - func fileExists(name string) bool: 检查文件是否存在
    - func validDbType(dbType string) bool ：对比数据库类型是否在支持的数据库列表中
    - func netName(chainParams *chaincfg.Params) string ：从chainParams参数中返回网络名字
    - func setupGlobalConfig() error ： 检查任何无效的条件的全局配置选项，并在最初的解析之后执行任何必要的附加设置。
- loadheaders.go
    - type headersCmd struct : 为loadheaders命令定义的配置选项，有Bulk，表示使用批量加载头文件还是是每次加载一个
    - func (cmd *headersCmd) Execute(args []string) error ：执行加载头文件
	    - 这个方法的逻辑很简单，数据库全局配置设置，加载数据库，加载相应的buket，一次性加载出blockIdxBucket，统计个数，遍历，通过tx.FetchBlockHeader加载区块头
	    - 创建一个chainhash.Hash的slice，遍历blockIdxBucket，添加到slice中，通过tx.FetchBlockHeaders加载区块头
- insecureimport.go
    - type importCmd struct ：为insecureimport命令定义配置选项，有InFile和Progress，前者表示包含块的文件，后者表示每次通过这个数秒时，显示一条进度消息——使用0来禁用进度公告
    - type importResults struct ：为一个import操作装统计和结果，有blocksProcessed,blocksImported,err,第一个表示处理过的块，第二个表示导入的块数量，第三个为错误结果
    - type blockImporter struct ：从区块数据库不断导入一个区块数据，装其中的信息，有db,r,processQueue,doneChan,errChan,quit,wg,blocksProcessed,blocksImported,receivedLogBlocks,receivedLogTx,lastHeight,lastBlockTime,lastLogTime
    - func (bi *blockImporter) readBlock() ([]byte, error) :从输入文件中读取下一个块
    - func (bi *blockImporter) processBlock(serializedBlock []byte) (bool, error) : 有可能将块导入到数据库。首先是对原始块进行反序列化，同时检查错误。已经知道的块就会跳过，如果是孤块就会被认为是错误的。在导入时，当出现任何可能的错误都会直接返回失败
    - func (bi *blockImporter) readHandler() :readHandler方法是从导入文件中读取块的主要处理方法。这个方法能允许块处理和块读取并行处理。它必须以一个goroutine的方式运行。
    - func (bi *blockImporter) logProgress() :对块处理进行日志处理是作为一个信息消息。为了防止消息过多，限制在每一个 importCfg.Progress秒内只有一个消息
    - func (bi *blockImporter) processHandler() ：proceshandler是处理块的主要处理程序。这使得块处理可以与导入文件中的块读取并行进行。它必须以一个goroutine的方式运行。
    - func (bi *blockImporter) statusHandler(resultsChan chan *importResults) ：statusHandler 等待来自导入操作的更新，并通过导入的结果通知已通过的doneChan。如果从其中任何一个错误报告，它也会导致所有goroutines退出。
    - func (bi *blockImporter) Import() chan *importResults : Import是核心功能，它处理从与块导入器相关联的文件导入到数据库的文件。它返回一个通道，当操作完成时，结果将返回。
    - func newBlockImporter(db database.DB, r io.ReadSeeker) *blockImporter : 为文件读取seeker和数据库创建一个新的导入句柄
    - func (cmd *importCmd) Execute(args []string) error ： 是命令的主要执行入口，由解析器调用
- signal.go
    - func mainInterruptHandler() : mainInterruputHandler在interruptChannel上侦听SIGINT（Ctrl+C）信号，并相应地调用注册的interruptcallback。它还监听回调注册。它必须以一个goroutine的方式运行。
    - func addInterruptHandler(handler func()) : 当收到SIGINT（Ctrl+C）信号，给调用程序添加一个中断处理
- fetchblock.go
    - type fetchBlockCmd struct : 给fetchblock命令定义的配置选项
    - func (cmd *fetchBlockCmd) Execute(args []string) error ： 是获取区块的主要执行方法，由解析器调起
    - func (cmd *fetchBlockCmd) Usage() string ：重写命令的用法显示
- fetchblockregion.go
    - type blockRegionCmd struct : 给fetchblockregion命令定义的配置选项
    - func (cmd *blockRegionCmd) Execute(args []string) error ：是获取区块区域的主要执行方法，由解析器调起
    - func (cmd *blockRegionCmd) Usage() string ：重写命令的用法显示

### interface.go

接口定义了DB接口，Tx接口，BlockRegion结构，Cursor接口，Bucket接口

    type Cursor interface {
    	Bucket()	Bucket
    	Delete()	error
    	First()	bool
    	Last()	bool
    	Next()	bool
    	Prev()	bool
    	Seek(seek []byte)	bool
    	Key()	[]byte
    	Value()	[]byte
    }

    type Bucket interface {
    	Bucket(key []byte) Bucket
    	CreateBucket(key []byte)(Bucket,error)
    	CreateBucketIfNotExists(key []byte)(Bucket,error)
    	DeleteBucket(key []byte) error
    	ForEach(func(k,v []byte) error) error
    	ForEachBucket(func(k []byte) error) error
    	Cursor() Cursor
    	Writable()	bool
    	Put(key,value []byte) error
    	Get(key []byte) []byte
    	Delete(key []byte) error
    }

    type BlockRegion struct {
    	Hash *chainhash.Hash
    	Offset	uint32
    	Len	uint32
    }

    type Tx interface {
    	Metadata()	Bucket
    	StoreBlock(block *btcutil.Block) error
    	HasBlock(hash *chainhash.Hash) (bool,error)
    	HasBlocks(hashes []chainhash.Hash) ([]bool,error)
    	FetchBlockHeader(hash *chainhash.Hash) ([]byte,error)
    	FetchBlockHeaders(hashes []chainhash.Hash) ([][]byte,error)
    	FetchBlock(hash *chainhash.Hash) ([]byte,error)
    	FetchBlocks(hashes []chainhash.Hash) ([][]byte,error)
    	FetchBlockRegion(region *BlockRegion) ([]byte,error)
    	FetchBlockRegions(regions []BlockRegion) ([][]byte,error)
    	Commit() error
    	Rollback() error
    }

    type DB interface {
    	Type() string
    	Begin(writable bool) (Tx,error)
    	View(fn func(tx Tx) error) error
    	Update(fn func(tx Tx) error) error
    	Close() error
    }

- Tx接口：
    - Metadata()，通过它可以获得根Bucket，所有的元数据均归属于Bucket，Bucket及其中的K/V对最终存于leveldb中。在一个Transaction中，对元数据的操作均是通过Metadata()得到Bucket后，再在Bucket中进行操作
    - ***Block***，与Block操作相关的接口，主要是通过读写文件来读写Block
    - Commit()和Rollback()方法，在可写Tx中写元数据或者区块后，均需要通过Commit()来提交修改并关闭Tx，或者通过Rollback来丢弃修改或关闭只读Tx，作用与BoltDB中的一致。


ffldb对上面的接口进行了实现，直接看其db的结构定义：

    type db struct {
    	writeLock sync.Mutex   // Limit to one write transaction at a time.
    	closeLock sync.RWMutex // Make database close block while txns active.
    	closed    bool         // Is the database closed?
    	store     *blockStore  // Handles read/writing blocks to flat files.
    	cache     *dbCache     // Cache layer which wraps underlying leveldb DB.
    }

各个属性的意义：

- writeLock：互斥锁，保证同时只有一个可写transaction;
- closeLock：保证数据库Close时所有已经打开的transaction均已结束;
- closed:指示数据库是否已经关闭;
- store:指向blockStore，用于读写区块;
- cache:指向dbCache，用于读写元数据;

db.go实现了database.DB接口，其中各个方法的实现跟BoltDB中基本类似，也是通过View()或者Update()的回调方法回去Tx对象或其引用，
然后调用Tx中接口进行数据库操作，因此就不重点分析db的各个方法实现，本身不是区块链技术的最核心部分，项目对于这一块底层采取拿来即用的策略。
对于Tx的实现，ffldb中transaction的定义如何，如何实现database.Tx接口：

    type transaction struct {
        managed        bool             // Is the transaction managed?
        closed         bool             // Is the transaction closed?
        writable       bool             // Is the transaction writable?
        db             *db              // DB instance the tx was created from.
        snapshot       *dbCacheSnapshot // Underlying snapshot for txns.
        metaBucket     *bucket          // The root metadata bucket.
        blockIdxBucket *bucket          // The block index bucket.

        // Blocks that need to be stored on commit.  The pendingBlocks map is
        // kept to allow quick lookups of pending data by block hash.
        pendingBlocks    map[chainhash.Hash]int
        pendingBlockData []pendingBlock

        // Keys that need to be stored or deleted on commit.
        pendingKeys   *treap.Mutable
        pendingRemove *treap.Mutable

        // Active iterators that need to be notified when the pending keys have
        // been updated so the cursors can properly handle updates to the
        // transaction state.
        activeIterLock sync.RWMutex
        activeIters    []*treap.Iterator
    }

其中各个属性的意义：

- managed:transaction是否被db托管，托管状态的transaction不能再主动调用Commit()或者Rollback()
- closed:指示当前transaction是否已经结束
- writable:指示当前transaction是否可写
- db:指向与当前transaction绑定的db对象
- snapshot:当前transaction读到的元数据缓存的一个快照，在transaction打开的时候对dbCache进行快照得到的，也是元数据存储中MVCC机制的一部分，类似于BoltDB中读meta page
- metaBucket:存储元数据的根Bucket
- blockIdxBucket:存储区块hash与其序号的Bucket，它是metaBucket的第一个子Bucket，且只在ffldb内部使用
- pendingBlocks:记录待提交Block的哈希与其在pendingBlockData中的位置的对应关系
- pendingBlockData:顺序记录所有待提交Block的字节序列
- pendingKeys:待添加或者更新的元数据集合，请注意，它指向一个树堆
- pendingRemove:待删除的元数据集合，它也指向一个树堆，与pendingKeys一样，它们均通过dbCache向leveldb中更新
- activeIterLock: 对activeIters的保护锁
- activeIters:用于记录当前transaction中查找dbCache的Iterators，当向dbCache中更新Key时，树堆旋转会更新节点间关系，故需将所有活跃的Iterator复位

transaction中主要有三大类方法，首先看看db.go中的Metadata()方法：

    // Metadata returns the top-most bucket for all metadata storage.
    // This function is part of the database.Tx interface implementation.
    func (tx *transaction) Metadata() database.Bucket {
    	return tx.metaBucket
    }

Metaata()方法返回根Bucket，剩下的操作均通过它来进行。只要看看bucket的定义：

    // bucket is an internal type used to represent a collection of key/value pairs
    // and implements the database.Bucket interface.
    type bucket struct {
    	tx *transaction
    	id [4]byte
    }

ffldb利用leveldb来存储K/V对，leveldb底层数据结构为LSM树（log-structed merge-tree）,而BoltDB采用B+ Tree.
ffldb利用leveldb提供的接口来读写K/V,而leveldb中没有Bucket的概念，也没有对Key进行分层管理的方法，那ffldb
总要实现Bucket，分析一下源码：

    // CreateBucket creates and returns a new nested bucket with the given key.
    func (b *bucket) CreateBucket(key []byte) (database.Bucket, error) {
    	// Ensure transaction state is valid.
    	...
    	// Ensure the transaction is writable.
    	...
    	// Ensure a key was provided.
    	...

    	// Ensure bucket does not already exist.
    	bidxKey := bucketIndexKey(b.id, key)
    	if b.tx.hasKey(bidxKey) {
    		str := "bucket already exists"
    		return nil, makeDbErr(database.ErrBucketExists, str, nil)
    	}

    	// Find the appropriate next bucket ID to use for the new bucket.  In the case of the special internal block index, keep the fixed ID.
    	var childID [4]byte
    	if b.id == metadataBucketID && bytes.Equal(key, blockIdxBucketName) {
    		childID = blockIdxBucketID
    	} else {
    		var err error
    		childID, err = b.tx.nextBucketID()
    		if err != nil {
    			return nil, err
    		}
    	}

    	// Add the new bucket to the bucket index.
    	if err := b.tx.putKey(bidxKey, childID[:]); err != nil {
    		str := fmt.Sprintf("failed to create bucket with key %q", key)
    		return nil, convertErr(str, err)
    	}
    	return &bucket{tx: b.tx, id: childID}, nil
    }

上面的方法主要是通过key来创建分层的Bucket：

- 1.通过 bidxKey := bucketIndexKey(b.id, key) 创建子Bucket的key
- 2.为子Bucket指定或选择一个id
- 3.将子Bucket的Key和id作为K/V记录存入父Bucket中

再来看一下bucketIndexKey方法，就能明白Bucket的定义：

    func bucketIndexKey(parentID [4]byte, key []byte) []byte {
    	// The serialized bucket index key format is:
    	//   <bucketindexprefix><parentbucketid><bucketname>
    	indexKey := make([]byte, len(bucketIndexPrefix)+4+len(key))
    	copy(indexKey, bucketIndexPrefix)
    	copy(indexKey[len(bucketIndexPrefix):], parentID[:])
    	copy(indexKey[len(bucketIndexPrefix)+4:], key)
    	return indexKey
    }

一个子Bucket的Key总是<bucketindexprefix><parentbucketid><bucketname> 这三部分构成，如果一个Bucket的Key是这种形式，那它对应
一个子Bucket，其Value装的就是Bucket的id。从上面的分析可以看出来，ffldb是通过Bucket的Key的分层形式来标记父子关系的。
BoltDB是B+树的形式，子Bucket对应一颗独立的B+树，当向子Bucket中添加K/V时，就是向对应的B+树中插入记录。

ffldb也是需要向子Bucket中添加K/V的，需要分析一下源码：

    func (b *bucket) Put(key, value []byte) error {
    	// Ensure transaction state is valid.
    	...
    	// Ensure the transaction is writable.
    	...
    	// Ensure a key was provided.
        ...

    	return b.tx.putKey(bucketizedKey(b.id, key), value)
    }

很明显，在经过对transaction和key的三个判断之后，直接就是putKey的方法，关键在于对key进行了一个处理，bucketizedKey(b.id, key)

    func bucketizedKey(bucketID [4]byte, key []byte) []byte {
    	// The serialized block index key format is:
    	//   <bucketid><key>
    	bKey := make([]byte, 4+len(key))
    	copy(bKey, bucketID[:])
    	copy(bKey[4:], key)
    	return bKey
    }

在向bucket中添加K/V时，Key会被转换成 <bucketid><key> 的形式，从而标记这一记录属于id为 bucketid的bucket。

ffldb中有两种Key的分层格式：

- bucket的Key的形式 <bucketindexprefix><parentbucketid><bucketname>
- bucket中元素的Key的形式 <bucketid><key>

两种分层格式，第一种分层格式清晰标记子Bucket的父子层级关系；第二种分层格式清晰标记Bucket中K/V对与Bucket的对应关系。

最终都是要写入leveldb数据库，其实K/V写入的时候并没有Bucket的概念，所有的key都是一种扁平结构。

但还是绑定了bucket，比如抽取数据需要用到的游标cursor，这个游标cursor就跟bucket进行了绑定，其是通过
leveldb中的Iterator实现的。

bucket的Put()方法分析，添加的K/V会通过transaction的PutKey()方法先加入到pendingKeys中

    func (tx *transaction) putKey(key, value []byte) error {
    	// Prevent the key from being deleted if it was previously scheduled
    	// to be deleted on transaction commit.
    	tx.pendingRemove.Delete(key)

    	// Add the key/value pair to the list to be written on transaction
    	// commit.
    	tx.pendingKeys.Put(key, value)
    	tx.notifyActiveIters()
    	return nil
    }

同样的逻辑，bucket的Delete()方法也是调用transaction的deleteKey()方法来实现。deleteKey()中，会将
要删除的key添加到pendingRemove中，待transaction Commit最终将pendingKeys添加到leveldb中，pendingRemove中
的Key将从leveldb中删除。

    func (c *cursor) Delete() error {
    	// Ensure transaction state is valid.
    	...
    	// Error if the cursor is exhausted.
    	...
    	// Do not allow buckets to be deleted via the cursor.
    	...

    	c.bucket.tx.deleteKey(copySlice(key), true)
    	return nil
    }

    func (tx *transaction) deleteKey(key []byte, notifyIterators bool) {
    	// Remove the key from the list of pendings keys to be written on transaction commit if needed.
    	tx.pendingKeys.Delete(key)

    	// Add the key to the list to be deleted on transaction	commit.
    	tx.pendingRemove.Put(key, nil)

    	// Notify the active iterators about the change if the flag is set.
    	if notifyIterators {
    		tx.notifyActiveIters()
    	}
    }

Get()方法也会最终调用transaction的fetchKey()方法来查询，fetchKey()先从pendingRemove或者pendingKeys
查找，如果找不到再从dbCache的一个快照中查找。

    func (b *bucket) Get(key []byte) []byte {
    	// Ensure transaction state is valid.
    	if err := b.tx.checkClosed(); err != nil {
    		return nil
    	}

    	// Nothing to return if there is no key.
    	if len(key) == 0 {
    		return nil
    	}

    	return b.tx.fetchKey(bucketizedKey(b.id, key))
    }

    func (tx *transaction) fetchKey(key []byte) []byte {
    	// When the transaction is writable, check the pending transaction
    	// state first.
    	if tx.writable {
    		if tx.pendingRemove.Has(key) {
    			return nil
    		}
    		if value := tx.pendingKeys.Get(key); value != nil {
    			return value
    		}
    	}

    	// Consult the database cache and underlying database.
    	return tx.snapshot.Get(key)
    }

transaction中第二类是读取Block相关的方法，先来重点分析一下StoreBlock()和FetchBlock()方法：

    func (tx *transaction) StoreBlock(block *btcutil.Block) error {
    	// Ensure transaction state is valid.
    	...
    	// Ensure the transaction is writable.
    	...
    	// Reject the block if it already exists.
    	...
    	blockBytes, err := block.Bytes()
    	...

    	if tx.pendingBlocks == nil {
    		tx.pendingBlocks = make(map[chainhash.Hash]int)
    	}
    	tx.pendingBlocks[*blockHash] = len(tx.pendingBlockData)
    	tx.pendingBlockData = append(tx.pendingBlockData, pendingBlock{
    		hash:  blockHash,
    		bytes: blockBytes,
    	})
    	log.Tracef("Added block %s to pending blocks", blockHash)

    	return nil
    }

StoreBlock()主要是先把block放到pendingBlockData数组中，等待Commit时写入文件。

    func (tx *transaction) FetchBlock(hash *chainhash.Hash) ([]byte, error) {
    	// Ensure transaction state is valid.
    	...
    	// When the block is pending to be written on commit return the bytes from there.
    	if idx, exists := tx.pendingBlocks[*hash]; exists {
    		return tx.pendingBlockData[idx].bytes, nil
    	}

    	// Lookup the location of the block in the files from the block index.
    	blockRow, err := tx.fetchBlockRow(hash)
    	if err != nil {
    		return nil, err
    	}
    	location := deserializeBlockLoc(blockRow)

    	// Read the block from the appropriate location.  The function also
    	// performs a checksum over the data to detect data corruption.
    	blockBytes, err := tx.db.store.readBlock(hash, location)
    	if err != nil {
    		return nil, err
    	}

    	return blockBytes, nil
    }

读Block时，先从pendingBlocks中查找，如果有则直接从pendingBlockData中返回；如果没有，就需要通过块hash先找到块的位置，
tx.fetchBlockRow(hash) 取到blockRow，反序列化成location，然后拿location和hash取出block。
上面的所谓block的位置，也就是block的metadata，对应的是block的索引。

总结：通过transaction读写数据或者Block，均会先对pendingBlocks或pendingKeys与pendingRemove读写，可以看作transaction的缓冲，
在Commit时被同步到文件或者leveldb中。Commit()最终会调用writePendingAndCommit()进行实际操作。

    func (tx *transaction) writePendingAndCommit() error {
    	// Save the current block store write position for potential rollback.
    	// These variables are only updated here in this function and there can only be one write transaction active at a time, so it's safe to store them for potential rollback.
    	wc := tx.db.store.writeCursor
    	wc.RLock()
    	oldBlkFileNum := wc.curFileNum
    	oldBlkOffset := wc.curOffset
    	wc.RUnlock()

    	// rollback is a closure that is used to rollback all writes to the block files.
    	rollback := func() {
    		// Rollback any modifications made to the block files if needed.
    		tx.db.store.handleRollback(oldBlkFileNum, oldBlkOffset)
    	}

    	// Loop through all of the pending blocks to store and write them.
    	for _, blockData := range tx.pendingBlockData {
    		log.Tracef("Storing block %s", blockData.hash)
    		location, err := tx.db.store.writeBlock(blockData.bytes)
    		if err != nil {
    			rollback()
    			return err
    		}

    		// Add a record in the block index for the block.
    		// The record includes the location information needed to locate the block on the filesystem as well as the block header since they areso commonly needed.
    		blockRow := serializeBlockLoc(location)
    		err = tx.blockIdxBucket.Put(blockData.hash[:], blockRow)
    		if err != nil {
    			rollback()
    			return err
    		}
    	}

    	// Update the metadata for the current write file and offset.
    	writeRow := serializeWriteRow(wc.curFileNum, wc.curOffset)
    	if err := tx.metaBucket.Put(writeLocKeyName, writeRow); err != nil {
    		rollback()
    		return convertErr("failed to store write cursor", err)
    	}

    	// Atomically update the database cache.  The cache automatically
    	// handles flushing to the underlying persistent storage database.
    	return tx.db.cache.commitTx(tx)
    }

在writePendingAndCommit()中，需要保存当前块的写的位置，以备回滚，当然这些变量只在这个函数中更新，并且一次只能有一个写事务活动，
所以将它们存储在潜在回滚中是安全的。

回滚是一个闭包函数，用于回滚在这个方法中所有已经写入文件的区块。

写文件主要三个方面：

- 通过blockStore将pendingBlockData中的区块遍历写入文件，tx.db.store.writeBlock(blockData.bytes)，同时将区块的hash与它在
文件中的位置写入blockIdxBucket，以便后续查找，tx.blockIdxBucket.Put(blockData.hash[:], blockRow)。
    - 在块索引中添加一个记录。blockData.hash[:]
    - 记录包括在文件系统上定位区块所需的位置信息，以及块头信息，因为它们通常是需要的。blockRow
- 更新metaBucket中记录当前文件读写位置的K/V，tx.metaBucket.Put(writeLocKeyName, writeRow)。
- 通过dbCache中commitTx()将待提交的K/V写入树堆缓存，必要时写入leveldb，tx.db.cache.commitTx(tx)。

#### blockStore

transaction中读写元数据或者区块时，最终会通过blockStore读写文件或者dbCache读写树堆或者leveldb。
接下来分析blockStore和dbCache。

看看glockio.go中的blockStore结构定义：

    type blockStore struct {
    	// 指示当前Block网络类型，比如MainNet，TestNet或者SimNet，在向文件中写入区块时，当然要指定该区块来自哪类网络.
    	network wire.BitcoinNet

    	// 存储Block的文件在磁盘上的存储路径
    	basePath string

    	// 存储Block文件的最大的Size
    	maxBlockFileSize uint32

    	// 对openBlockFiles进行保护的读写锁
    	obfMutex         sync.RWMutex
    	// 对openBlocksLRU和fileNumTolRUElem进行保护的互斥锁
    	lruMutex         sync.Mutex
    	// 已打开文件的序号和LRU列表，默认的最大打开文件数是25
    	openBlocksLRU    *list.List // Contains uint32 block file numbers.
    	// 记录文件序号与openBlocksLRU中元素的对应关系
    	fileNumToLRUElem map[uint32]*list.Element
    	// 记录所有打开的只读文件的序号与文件指针的对应关系
    	openBlockFiles   map[uint32]*lockableFile

    	// 指向当前写入的文件，记录其文件序号和写偏移
    	writeCursor *writeCursor

    	//  openFile、openWriteFile和deleteFile三个方法的接口方法
    	openFileFunc      func(fileNum uint32) (*lockableFile, error)
    	openWriteFileFunc func(fileNum uint32) (filer, error)
    	deleteFileFunc    func(fileNum uint32) error
    }

blockStore结构的定义是非常复杂的，通过blockStore的readBlock()和writeBlock()方法来了解其工作机制。

    func (s *blockStore) readBlock(hash *chainhash.Hash, loc blockLocation) ([]byte, error) {
    	// 根据需要获取所引用的块文件句柄打开文件。该函数还可以根据需要处理关闭文件，以避免越过最大允许打开的文件。
    	blockFile, err := s.blockFile(loc.blockFileNum)
    	if err != nil {
    		return nil, err
    	}

    	serializedData := make([]byte, loc.blockLen)
    	n, err := blockFile.file.ReadAt(serializedData, int64(loc.fileOffset))
    	blockFile.RUnlock()
    	if err != nil {
    		str := fmt.Sprintf("failed to read block %s from file %d, "+
    			"offset %d: %v", hash, loc.blockFileNum, loc.fileOffset,
    			err)
    		return nil, makeDbErr(database.ErrDriverSpecific, str, err)
    	}

    	// 计算读取数据的校验和，并确保它与序列化的校验和匹配。这将检测到平面文件中的任何数据损坏，而不需要在加载的块上做更昂贵的merkle根计算。
    	serializedChecksum := binary.BigEndian.Uint32(serializedData[n-4:])
    	calculatedChecksum := crc32.Checksum(serializedData[:n-4], castagnoli)
    	if serializedChecksum != calculatedChecksum {
    		str := fmt.Sprintf("block data for block %s checksum "+
    			"does not match - got %x, want %x", hash,
    			calculatedChecksum, serializedChecksum)
    		return nil, makeDbErr(database.ErrCorruption, str, nil)
    	}

    	// 与区块相关联的网络必须与当前的活动网络相匹配，否则某人可能会将区块文件放在目录中错误的网络中。
    	serializedNet := byteOrder.Uint32(serializedData[:4])
    	if serializedNet != uint32(s.network) {
    		str := fmt.Sprintf("block data for block %s is for the "+
    			"wrong network - got %d, want %d", hash, serializedNet,
    			uint32(s.network))
    		return nil, makeDbErr(database.ErrDriverSpecific, str, nil)
    	}

    	// 原始块不包括网络、块的长度和校验和。
    	return serializedData[8 : n-4], nil
    }

readBlock()的实现步骤为：

- 通过blockStore的blockFile()查询已经打开的文件或者新打开一个文件
- 通过file.ReadAt()方法从文件中的loc.fileOffset位置读出区块数据，它的格式是“<network><block length><serialized block><checksum>”
- 从区块数据中解析出block的字节流

其中最重要的是通过blockFile()得到一个文件句柄：

    func (s *blockStore) blockFile(fileNum uint32) (*lockableFile, error) {
    	// 当被请求的块文件打开时，返回它。
    	wc := s.writeCursor
    	wc.RLock()
    	if fileNum == wc.curFileNum && wc.curFile.file != nil {
    		obf := wc.curFile
    		obf.RLock()
    		wc.RUnlock()
    		return obf, nil
    	}
    	wc.RUnlock()

    	// 尝试在整个文件读取锁下返回一个打开的文件。
    	s.obfMutex.RLock()
    	if obf, ok := s.openBlockFiles[fileNum]; ok {
    		s.lruMutex.Lock()
    		s.openBlocksLRU.MoveToFront(s.fileNumToLRUElem[fileNum])
    		s.lruMutex.Unlock()

    		obf.RLock()
    		s.obfMutex.RUnlock()
    		return obf, nil
    	}
    	s.obfMutex.RUnlock()

    	// 由于该文件还没有打开，所以需要在写锁下再次检查打开块文件映射，以防多个阅读器在这里出现，而另一个则已经打开了文件。
    	s.obfMutex.Lock()
    	if obf, ok := s.openBlockFiles[fileNum]; ok {
    		obf.RLock()
    		s.obfMutex.Unlock()
    		return obf, nil
    	}

    	// 该文件不是打开的，所以打开它，可能会在需要的时候关闭最近使用的文件。
    	obf, err := s.openFileFunc(fileNum)
    	if err != nil {
    		s.obfMutex.Unlock()
    		return nil, err
    	}
    	obf.RLock()
    	s.obfMutex.Unlock()
    	return obf, nil
    }

其实现步骤如下：

- 检查要查找的文件是否是writeCursor指向的文件，如果是则直接返回。
- 接着，从blockStore记录的openBlockFiles中查找文件，如果找到，将文件移至LRU列表的首位置，同时获得文件读锁后返回。
- s.obfMutex.Lock()  获取s.obfMutex的写锁并再次从openBlockFiles中查找文件
- 如果openBlockFiles中找不到目标文件，就调用openFile()打开新文件

    func (s *blockStore) openFile(fileNum uint32) (*lockableFile, error) {
    	// Open the appropriate file as read-only.
    	filePath := blockFilePath(s.basePath, fileNum)
    	file, err := os.Open(filePath)
    	if err != nil {
    		return nil, makeDbErr(database.ErrDriverSpecific, err.Error(),
    			err)
    	}
    	blockFile := &lockableFile{file: file}

    	s.lruMutex.Lock()
    	lruList := s.openBlocksLRU
    	if lruList.Len() >= maxOpenFiles {
    		lruFileNum := lruList.Remove(lruList.Back()).(uint32)
    		oldBlockFile := s.openBlockFiles[lruFileNum]

    		// Close the old file under the write lock for the file in case
    		// any readers are currently reading from it so it's not closed
    		// out from under them.
    		oldBlockFile.Lock()
    		_ = oldBlockFile.file.Close()
    		oldBlockFile.Unlock()

    		delete(s.openBlockFiles, lruFileNum)
    		delete(s.fileNumToLRUElem, lruFileNum)
    	}
    	s.fileNumToLRUElem[fileNum] = lruList.PushFront(fileNum)
    	s.lruMutex.Unlock()

    	// Store a reference to it in the open block files map.
    	s.openBlockFiles[fileNum] = blockFile

    	return blockFile, nil
    }

openFile()中主要执行：

- 1.直接通过os.Open()调用以只读模式打开目标文件
- 2.检测openBlocksLRU是否已满，如果已满，则将列表末尾元素移除，同时将对应的文件关闭并从openBlockFiles将其移除，然后将新打开的文件添加到列表首位置；其中对openBlocksLRU和fileNumLRUElem的访问均在s.lruMutex保护下;
- 3.将新打开的文件放入openBlockFiles中

从openFile()中可以看到，blockStore通过openBlockFiles和openBlocksLRU及fileNumToLRUElem维护了一个已经打开的只读文件的LRU缓存列表，可以加速从文件读取区块。

writeBlock()：

    func (s *blockStore) writeBlock(rawBlock []byte) (blockLocation, error) {
    	// Compute how many bytes will be written.
    	// 4 bytes each for block network + 4 bytes for block length +
    	// length of raw block + 4 bytes for checksum.
    	blockLen := uint32(len(rawBlock))
    	fullLen := blockLen + 12

    	// Move to the next block file if adding the new block would exceed the
    	// max allowed size for the current block file.  Also detect overflow
    	// to be paranoid, even though it isn't possible currently, numbers
    	// might change in the future to make it possible.
    	//
    	// NOTE: The writeCursor.offset field isn't protected by the mutex
    	// since it's only read/changed during this function which can only be
    	// called during a write transaction, of which there can be only one at
    	// a time.
    	wc := s.writeCursor
    	finalOffset := wc.curOffset + fullLen
    	if finalOffset < wc.curOffset || finalOffset > s.maxBlockFileSize {
    		// This is done under the write cursor lock since the curFileNum
    		// field is accessed elsewhere by readers.
    		//
    		// Close the current write file to force a read-only reopen
    		// with LRU tracking.  The close is done under the write lock
    		// for the file to prevent it from being closed out from under
    		// any readers currently reading from it.
    		wc.Lock()
    		wc.curFile.Lock()
    		if wc.curFile.file != nil {
    			_ = wc.curFile.file.Close()
    			wc.curFile.file = nil
    		}
    		wc.curFile.Unlock()

    		// Start writes into next file.
    		wc.curFileNum++
    		wc.curOffset = 0
    		wc.Unlock()
    	}

    	// All writes are done under the write lock for the file to ensure any
    	// readers are finished and blocked first.
    	wc.curFile.Lock()
    	defer wc.curFile.Unlock()

    	// Open the current file if needed.  This will typically only be the
    	// case when moving to the next file to write to or on initial database
    	// load.  However, it might also be the case if rollbacks happened after
    	// file writes started during a transaction commit.
    	if wc.curFile.file == nil {
    		file, err := s.openWriteFileFunc(wc.curFileNum)
    		if err != nil {
    			return blockLocation{}, err
    		}
    		wc.curFile.file = file
    	}

    	// Bitcoin network.
    	origOffset := wc.curOffset
    	hasher := crc32.New(castagnoli)
    	var scratch [4]byte
    	byteOrder.PutUint32(scratch[:], uint32(s.network))
    	if err := s.writeData(scratch[:], "network"); err != nil {
    		return blockLocation{}, err
    	}
    	_, _ = hasher.Write(scratch[:])

    	// Block length.
    	byteOrder.PutUint32(scratch[:], blockLen)
    	if err := s.writeData(scratch[:], "block length"); err != nil {
    		return blockLocation{}, err
    	}
    	_, _ = hasher.Write(scratch[:])

    	// Serialized block.
    	if err := s.writeData(rawBlock[:], "block"); err != nil {
    		return blockLocation{}, err
    	}
    	_, _ = hasher.Write(rawBlock)

    	// Castagnoli CRC-32 as a checksum of all the previous.
    	if err := s.writeData(hasher.Sum(nil), "checksum"); err != nil {
    		return blockLocation{}, err
    	}

    	loc := blockLocation{
    		blockFileNum: wc.curFileNum,
    		fileOffset:   origOffset,
    		blockLen:     fullLen,
    	}
    	return loc, nil
    }

其主要步骤为：

- 1.检查写入区块后是否超过文件大小限制，如果超过，则关闭当前文件，创建一个新文件；否则，直接在当前文件的wc.curOffset偏移处开始写区块
- 2.wc.curFile.Lock() 关闭writeCursor指向的文件，在调用Close()之前，获取了lockableFile的写锁，以防其他线程正在读该文件
- 3.wc.curFileNum++ 将writeCursor指向下一个文件 wc.curOffset = 0 将文件内偏移复位
- 4.file, err := s.openWriteFileFunc(wc.curFileNum)  调用openWriteFile()以可读可写方式打开或者创建一个新的文件，同时将writeCursor指向该文件
- 5.origOffset := wc.curOffset  记录下写区块的文件内起始偏移位置，随后开始向文件中写区块数据
- 6.依次向文件中写入网络号，区块长度值，区块数据和前三项的crc32检验和，存在文件上的区块封装格式为：<network><block length><serialized block><checksum>
- 7.loc := blockLocation 创建被写入区块对应的blockLocation对象，它由存储区块的文件的序号，区块存储位置在该文件内的起始偏移及封装后的区块长度构成，最后返回该blockLocation对象

总结：通过readBlock()和writeBlock()，基本上可以了解blockStore的整个工作机制，主要是通过一个LRU列表来管理已经打开的
只读文件，并通过writeCursor来记录当前写入的文件及文件内偏移，在写入区块时，如果写入区块后超过了设置的最大文件Size，
则新建一个文件写入。

### dbCache

    type dbCache struct {
    	// 指向leveldb的DB对象，用于向leveldb中存取K/V
    	ldb *leveldb.DB

    	// 指向当前db下的blockStore，用于向leveldb中写元数据之前，通过blockStore将区块缓存强制写入磁盘
    	store *blockStore

        // 简单地讲，它是缓存的待添加和删除的元数据的总大小限制，默认值为100M;
    	maxSize       uint64
    	// 向leveldb中写数据的时间间隔;
    	flushInterval time.Duration
    	// 上次向leveldb中写数据的时间戳;
    	lastFlush     time.Time

        // 对cachedKeys和cachedRemove进行读写保护，它们会在dbCache向leveldb写数据时更新，在dbCache快照时被读取;
    	cacheLock    sync.RWMutex
    	// 缓存待添加的Key，它指向一个树堆;
    	cachedKeys   *treap.Immutable
    	// 缓存待删除的Key，它也指向一个树堆，请注意，cachedKeys和cachedRemove与transaction中的pendingKeys和pendingRemove有区别
    	cachedRemove *treap.Immutable
    }

在db.go文件中writePendingAndCommit()方法中看到transaction提交的最后一步是return tx.db.cache.commitTx(tx)
调用dbCache的commitTx()来提交元数据的更新，来看看commitTx()：

    func (c *dbCache) commitTx(tx *transaction) error {
    	// Flush the cache and write the current transaction directly to the
    	// database if a flush is needed.
    	if c.needsFlush(tx) {
    		if err := c.flush(); err != nil {
    			return err
    		}

    		// Perform all leveldb updates using an atomic transaction.
    		err := c.commitTreaps(tx.pendingKeys, tx.pendingRemove)
    		if err != nil {
    			return err
    		}

    		// Clear the transaction entries since they have been committed.
    		tx.pendingKeys = nil
    		tx.pendingRemove = nil
    		return nil
    	}

    	// At this point a database flush is not needed, so atomically commit
    	// the transaction to the cache.

    	// Since the cached keys to be added and removed use an immutable treap,
    	// a snapshot is simply obtaining the root of the tree under the lock
    	// which is used to atomically swap the root.
    	c.cacheLock.RLock()
    	newCachedKeys := c.cachedKeys
    	newCachedRemove := c.cachedRemove
    	c.cacheLock.RUnlock()

    	// Apply every key to add in the database transaction to the cache.
    	tx.pendingKeys.ForEach(func(k, v []byte) bool {
    		newCachedRemove = newCachedRemove.Delete(k)
    		newCachedKeys = newCachedKeys.Put(k, v)
    		return true
    	})
    	tx.pendingKeys = nil

    	// Apply every key to remove in the database transaction to the cache.
    	tx.pendingRemove.ForEach(func(k, v []byte) bool {
    		newCachedKeys = newCachedKeys.Delete(k)
    		newCachedRemove = newCachedRemove.Put(k, nil)
    		return true
    	})
    	tx.pendingRemove = nil

    	// Atomically replace the immutable treaps which hold the cached keys to
    	// add and delete.
    	c.cacheLock.Lock()
    	c.cachedKeys = newCachedKeys
    	c.cachedRemove = newCachedRemove
    	c.cacheLock.Unlock()
    	return nil
    }

其中主要的步骤为：

- 1.如果离上一次flush已经超过一个刷新周期且dbCache已满，则调用flush()将树堆中的缓存写入leveldb，并将transaction中的待添加和移除的Keys通过commitTreaps()方法直接写入leveldb，写完后清空pendingKeys和pendingRemove；
- 2.如果不需要flush，则将transaction中的pendingKeys添加到newCachedKeys中，将pendingRemove添加到newCachedRemove
- 3.c.cachedKeys = newCachedKeys    最后更新dbCache中的cachedKeys和cachedRemove

#### treap

dbCache缓存Key，刷新缓存及读缓存的过程，dbCache中用于实际缓存的数据结构是treap.Immutable，它是dbCache的核心。

btcd中的treap既实现了Immutable版本，同时也提供了Muttable版本。

对于想了解treap的读者，可以先简单读一下[《随机平衡二叉查找树Treap的分析与应用》](https://www.byvoid.com/upload/wp/2010/12/treap-analysis-and-application.pdf)

简单说清楚treap：是tree和head的集合，二叉树与堆的结合，为了防止有序的序列在二叉树上退化成链，即在二叉树的节点中引入一个随机数，
用于对节点进行堆排序，让二叉查找树实现动态平平衡，同事形成最大堆和最小堆，从而保证其平衡性。树堆查找的时间复杂度都是O(logN)。

重点分析treap的Mutable和Immutable的Put()方法来了解treap的构建，添加节点后的旋转及Immutable的写时复制等过程。

    type Mutable struct {
    	root  *treapNode
    	count int

    	// totalSize is the best estimate of the total size of of all data in
    	// the treap including the keys, values, and node sizes.
    	totalSize uint64
    }

    type Immutable struct {
    	root  *treapNode
    	count int

    	// totalSize is the best estimate of the total size of of all data in
    	// the treap including the keys, values, and node sizes.
    	totalSize uint64
    }

Immutable和Mutable的定义完全一样，区别在于Immutable提供了写时复制，在Put()方法中就能看出其区别。其中的root属性指向treap的根节点

节点的结构为：

    type treapNode struct {
    	key      []byte
    	value    []byte
    	priority int
    	left     *treapNode
    	right    *treapNode
    }

treapNode中的key和value就是树堆节点的值，priority是用于构建堆的随机修正值，也叫节点的优先级。left和right分别指向左右子树根节点。

看看Mubtable的Put()方法：

    func (t *Mutable) Put(key, value []byte) {
    	// Use an empty byte slice for the value when none was provided.  This
    	// ultimately allows key existence to be determined from the value since
    	// an empty byte slice is distinguishable from nil.
    	if value == nil {
    		value = emptySlice
    	}

    	// The node is the root of the tree if there isn't already one.
    	if t.root == nil {
    		node := newTreapNode(key, value, rand.Int())
    		t.count = 1
    		t.totalSize = nodeSize(node)
    		t.root = node
    		return
    	}

    	// Find the binary tree insertion point and construct a list of parents
    	// while doing so.  When the key matches an entry already in the treap,
    	// just update its value and return.
    	var parents parentStack
    	var compareResult int
    	for node := t.root; node != nil; {
    		parents.Push(node)
    		compareResult = bytes.Compare(key, node.key)
    		if compareResult < 0 {
    			node = node.left
    			continue
    		}
    		if compareResult > 0 {
    			node = node.right
    			continue
    		}

    		// The key already exists, so update its value.
    		t.totalSize -= uint64(len(node.value))
    		t.totalSize += uint64(len(value))
    		node.value = value
    		return
    	}

    	// Link the new node into the binary tree in the correct position.
    	node := newTreapNode(key, value, rand.Int())
    	t.count++
    	t.totalSize += nodeSize(node)
    	parent := parents.At(0)
    	if compareResult < 0 {
    		parent.left = node
    	} else {
    		parent.right = node
    	}

    	// Perform any rotations needed to maintain the min-heap.
    	for parents.Len() > 0 {
    		// There is nothing left to do when the node's priority is
    		// greater than or equal to its parent's priority.
    		parent = parents.Pop()
    		if node.priority >= parent.priority {
    			break
    		}

    		// Perform a right rotation if the node is on the left side or
    		// a left rotation if the node is on the right side.
    		if parent.left == node {
    			node.right, parent.left = parent, node.right
    		} else {
    			node.left, parent.right = parent, node.left
    		}
    		t.relinkGrandparent(node, parent, parents.At(0))
    	}
    }

其中的主要步骤：

- 1.对于空树，添加的第一个节点直接成为根节点，可以看到，节点的priority是由rand.Int()生成的随机整数;
- 2.对于非空树，根据key来查找待插入的位置，并通过parentStack来记录查找路径。从根节点开始，如果待插入的Key小于根的Key，则进入左子树继续查找；如果待插入的Key大于根的Key，则进入右子树继续查找；如果待插入的Key正好的当前节点的Key，则直接更新其Value;
- 3.当树中没有找到Key，则应插入新的节点，此时parents中的最后一个节点就是新节点的父节点，请注意，parents.At(0)是查找路径上的最后一个节点。如果待插入的Key小于父节点的Key，则新节点变成父节点的左子节点，parent.left = node；否则，成为右子节点，parent.right = node;
- 4.由于新节点的priority是随机产生的，它插入树中后，树可能不满足最小堆性质了，所以接下来需要进行旋转。旋转过程需要向上递归进行直到整颗树满足最小难序。if node.priority >= parent.priority，如果新节点的优化级正好大于或者等于父节点的优先级，则不用旋转，树已经满足最小难序；如果新节点的优化级小于父节点的优化级，则需要旋转，将父节点变成新节点的子节点。如果新节点是父节点的左子节点，则需要进行右旋；如果新节点是父节点的右子节点，则需要进行左旋;
- 5.进行左旋或右旋后，原父节点变成新节点的子节点，但祖节点(原父节点的父节点)的子节点还指向原父节点，relinkGrandparent()将继续完成旋转过程。if grandparent == nil 如果祖节点是空，则说明原父节点就是树的根，不需要调整直接将新节点变成树的根即可；grandparent.left = node grandparent.right = node实际上是将新节点替代原父节点，变成祖节点的左子节点或者右子节点;
- 6.新节点、原父节点、祖节点完成旋转后，新节点变成了新的父节点，原交节点变成子节点，祖节点不变，但此时新节点的优化级可能还大于祖节点的优化级，则新的父节点、祖节点及祖节点的父节点还要继续旋转，这一过程向上递归到根节点，保证查找路径上节点均满足最小堆序，才完成了整个旋转过程及新节点插入过程。

从Mutable的Put()方法中，我们可以完整地了解treap的构建、插入及涉及到的子树旋转过程。Immutable的Put()与Mutable的Put()实现步骤大致一致，不同的是，Immutable没有直接修改原节点或旋转原树，而是将查找路径上的所有节点均复制一份出来与原树的其它节点一起形成一颗新的树，在新树上进行更新或者旋转后返回新树。

### driver.go

    type Driver struct {
    	DbType	string
    	Create func(args ...interface{}) (DB,error)
    	Open func(args ...interface{}) (DB,error)
    	UseLogger func(logger btclog.Logger)
    }

作为一个继承了DB接口的后端来进行注册，包括了属性dbType，然后有创建驱动和打开驱动的接口方法，还有日志记录器

项目代码地址： https://github.com/jianhuaixie/blockchain-buildwheels/tree/master/content/wheels-8