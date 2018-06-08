# 持久化和命令行接口

已经构建出一个PoW机制的区块链，但区块链的数据需要持久化到一个数据库，还需要提供一个简单的命令行接口，用户完成一些
与区块链的交互操作，既然认为区块链本质上是一个分布式数据库，那么就要完成存储和读取。

### 选择数据库

原则上，选用什么数据都是可以的，但go语言我们选择BoltDB

### BoltDB

- 简洁
- go实现
- 不需要运行一个服务器
- 能够允许构造想要的数据结构

Bolt使用键值存储，没有像SQL RDBMS的表，没有行和列。数据被存储为键值对。键值对被存储在bucket中，这是为了将相似的键值对
进行分组。因此，为了获取一个值，需要知道一个bucket和一个键（key）。

Bolt数据库没有数据类型：键和值都是字节数组（byte array）。需要存储go的Block，就需要进行序列化。实现一个从go struct转换
到一个byte array的机制，同时还要能转回struct。在这里我们选用go标准库 encoding/gob来完成这一目标。

### 数据库结构

在进行序列化存储之前，我们想要搞明白，到底什么数据存储到数据库中。

首先，我们看看Bitcoin是如何做的：

Bitcoin Core使用两个“bucket”来存储数据：

- blocks，存储了描述一条链中所有块的元数据
- chainstate，存储了一条链的状态，当前所有的未花费和交易输出，和一些元数据

在此版轮子中，还未进行交易，只需要blocks bucket。也简单将整个数据库存储为单个文件，而没有将区块存储在不同的文件中。
也不需要文件编号（file number）相关的东西，我们会用到的键值对有：

- 32字节的block-hash  -> block结构
- l -> 链中最后一个块的hash

### 持久化

从前一个轮子的NewBlockchain函数开始，此函数创建一个新的区块链实例，并且会添加一个创世块。加入数据库功能后，我们希望
做更多的事情：

- 1.打开一个数据库文件
- 2.检查文件里面是否已经存储了一个区块链
- 3.如果已经存储了一个区块链：
    - i.创建一个新的Blockchain实例
    - ii.设置Blockchain实例的tip为数据库中存储的最后一个块的哈希
- 4.如果没有区块链
    - i.创建创世块
    - ii.存储到数据库
    - iii.将创世块哈希保存为最后一个块的哈希
    - iv.创建一个新的Blockchain实例，初始时tip指向创世块

### 检查区块链

首先构造一个能遍历区块的区块链迭代器（BlockchainIterator），迭代器的初始状态为链中的tip，然后从尾到头（创世块）进行
迭代获取区块。实际上，选择一个tip就是意味着给一条链“投票”，怎么解释呢，一条链可能有很多分支，最长的那条链会被认为
是主分支，获得一个tip（可以是链中任意一个块）后，就可以重新构造整条链，所以说，一个tip就是区块链的一种标识符。

### 命令行接口

说有相关命令在这个版本的轮子中，都会通过CLI struct进行处理

    type CLI struct{
        blockchain *Blockchain
    }

### go 命令

    go build -o blockchain_go

    ./blockchain_go printchain
    ./blockchain_go addblock -data "Send 1 BTC to Silver"
    ./blockchain_go printchain

### 项目代码分为下面几个部分

block.go

    package main

    import (
    	"bytes"
    	"encoding/gob"
    	"log"
    	"time"
    )

    type Block struct {
    	Timestamp	int64
    	Data	[]byte
    	PrevBlockHash	[]byte
    	Hash	[]byte
    	Nonce	int
    }

    // 将Block序列化为一个字节数组
    func (block *Block) Serialize() []byte{
    	var result bytes.Buffer
    	encoder := gob.NewEncoder(&result)
    	err := encoder.Encode(block)
    	if err!=nil{
    		log.Panic(err)
    	}
    	return result.Bytes()
    }
    // 将字节数组反序列化为一个Block
    func DeserializeBlock(d []byte) *Block{
    	var block Block
    	decoder := gob.NewDecoder(bytes.NewReader(d))
    	err := decoder.Decode(&block)
    	if err!=nil{
    		log.Panic(err)
    	}
    	return &block
    }

    func NewBlock(data string,prevBlockHash []byte) *Block {
    	block := &Block{
    		Timestamp:time.Now().Unix(),
    		Data:[]byte(data),
    		PrevBlockHash:prevBlockHash,
    		Hash:[]byte{},
    		Nonce:0}
    	pow := NewProofOfWork(block)
    	nonce,hash := pow.Run()
    	block.Hash = hash[:]
    	block.Nonce = nonce
    	return block
    }

    func NewGenesisBlock() *Block {
    	return NewBlock("Genesis Block", []byte{})
    }

blockchain.go

    package main

    import (
    	"github.com/boltdb/bolt"
    	"log"
    	"fmt"
    )

    const dbFile = "blockchain.db"
    const blocksBucket = "blocks"

    // tip 尾部的意思，这里是存储最后一个块的hash值 ,存储最后的tip就能推导出整条chain
    // 在链的尾端可能会短暂分叉的情况，所以选择tip其实是选择那条链
    // db 存储数据库连接
    type Blockchain struct {
    	tip []byte
    	db *bolt.DB
    }

    func NewBlockchain() *Blockchain{
    	var tip []byte
    	// 打开一个BoltDB文件
    	db,err := bolt.Open(dbFile,0600,nil)
    	if err!=nil{
    		log.Panic(err)
    	}
    	err = db.Update(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		// 如果数据库中不存在区块链就创建一个，否则直接读取最后一个块的hash值
    		if bucket==nil{
    			fmt.Println("No existing blockchain found. Creating a new one...")
    			genesis := NewGenesisBlock()
    			bucket,err := tx.CreateBucket([]byte(blocksBucket))
    			if err!=nil{
    				log.Panic(err)
    			}
    			err = bucket.Put(genesis.Hash, genesis.Serialize())
    			if err!=nil{
    				log.Panic(err)
    			}
    			err = bucket.Put([]byte("1"),genesis.Hash)
    			if err!=nil{
    				log.Panic(err)
    			}
    			tip = genesis.Hash
    		}else{
    			tip = bucket.Get([]byte("1"))
    		}
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	blockchain := Blockchain{tip,db}
    	return &blockchain
    }

    // 加入区块时，需要将区块持久化到数据库中
    func (blockchain *Blockchain) AddBlock(data string){
    	var lastHash []byte
    	// 首先获取最后一个块的哈希用于生成新的哈希
    	err := blockchain.db.View(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		lastHash = bucket.Get([]byte("1"))
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	newBlock := NewBlock(data,lastHash)
    	err = blockchain.db.Update(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		err := bucket.Put(newBlock.Hash,newBlock.Serialize())
    		if err != nil {
    			log.Panic(err)
    		}
    		err = bucket.Put([]byte("1"),newBlock.Hash)
    		if err != nil {
    			log.Panic(err)
    		}
    		blockchain.tip = newBlock.Hash
    		return nil
    	})
    }

    type BlockchainIterator struct {
    	currentHash	[]byte
    	db	*bolt.DB
    }

    func (blockchain *Blockchain) Iterator() *BlockchainIterator{
    	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
    	return blockchainiterator
    }

    // 返回链中的下一个块
    func (i *BlockchainIterator) Next() *Block{
    	var block *Block
    	err := i.db.View(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		encodedBlock := bucket.Get(i.currentHash)
    		block = DeserializeBlock(encodedBlock)
    		return nil
    	})
    	if err!=nil{
    		log.Panic(err)
    	}
    	i.currentHash = block.PrevBlockHash
    	return block
    }

cli.go

    package main

    import (
    	"fmt"
    	"os"
    	"flag"
    	"log"
    )

    type CLI struct {
    	blockchain *Blockchain
    }
    const usage = `
    Usage:
    	addblock -data BLOCK_DATA	add a block to the blockchain
    	printchain	  print all the blocks of the blockchain
    `
    func (cli *CLI) printUsage(){
    	fmt.Println(usage)
    }
    func (cli *CLI) validateArgs(){
    	if len(os.Args)<2{
    		cli.printUsage()
    		os.Exit(1)
    	}
    }
    func (cli *CLI) Run(){
    	cli.validateArgs()
    	addBlockCmd := flag.NewFlagSet("addblock",flag.ExitOnError)
    	printChainCmd := flag.NewFlagSet("printchain",flag.ExitOnError)
    	addBlockData := addBlockCmd.String("data","","Block data")
    	switch os.Args[1]{
    	case "addblock":
    		err := addBlockCmd.Parse(os.Args[2:])
    		if err != nil{
    			log.Panic(err)
    		}
    	case	"printchain":
    		err := printChainCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	default:
    		cli.printUsage()
    		os.Exit(1)
    	}
    	if addBlockCmd.Parsed(){
    		if *addBlockData == ""{
    			addBlockCmd.Usage()
    			os.Exit(1)
    		}
    		cli.blockchain.AddBlock(*addBlockData)
    	}
    	if printChainCmd.Parsed(){
    		cli.printChain()
    	}
    }

commands.go

    package main

    import (
    	"fmt"
    	"strconv"
    )

    func (cli *CLI) addBlock(data string){
    	cli.blockchain.AddBlock(data)
    	fmt.Println("add block success!")
    }
    func (cli *CLI) printChain(){
    	blockchainiterator := cli.blockchain.Iterator()
    	for {
    		block := blockchainiterator.Next()
    		fmt.Printf("Prev hash: %x\n", block.PrevBlockHash)
    		fmt.Printf("Data: %s\n", block.Data)
    		fmt.Printf("Hash: %x\n", block.Hash)
    		pow := NewProofOfWork(block)
    		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
    		fmt.Println()
    		if len(block.PrevBlockHash) == 0{
    			break
    		}
    	}
    }

main.go

    package main

    func main()  {
    	blockchain := NewBlockchain()
    	defer blockchain.db.Close()
    	cli := CLI{blockchain}
    	cli.Run()
    }

proofofwork.go

    package main

    import (
    	"math"
    	"math/big"
    	"bytes"
    	"fmt"
    	"crypto/sha256"
    )

    const targetBits = 24
    var (
    	maxNonce = math.MaxInt64
    )
    type ProofOfWork struct {
    	block *Block
    	target *big.Int
    }
    func NewProofOfWork(block *Block) *ProofOfWork{
    	target := big.NewInt(1)
    	target.Lsh(target,uint(256-targetBits))
    	pow := &ProofOfWork{block,target}
    	return pow
    }

    func (pow *ProofOfWork) prepareData(nonce int) []byte{
    	data := bytes.Join(
    		[][]byte{
    			pow.block.PrevBlockHash,
    			pow.block.Data,
    			IntToHex(pow.block.Timestamp),
    			IntToHex(int64(targetBits)),
    			IntToHex(int64(nonce)),
    		},
    		[]byte{},
    	)
    	return data
    }

    func (pow *ProofOfWork) Run() (int,[]byte){
    	var hashInt big.Int
    	var hash [32]byte
    	nonce := 0
    	fmt.Printf("Mining the block containing \"%s\"\n", pow.block.Data)
    	for nonce<maxNonce{
    		data := pow.prepareData(nonce)
    		hash = sha256.Sum256(data)
    		hashInt.SetBytes(hash[:])
    		if hashInt.Cmp(pow.target)== -1{
    			fmt.Printf("\r%x", hash)
    			break
    		}else{
    			nonce++
    		}
    	}
    	fmt.Print("\n\n")
    	return nonce,hash[:]
    }

    func (pow *ProofOfWork) Validate() bool{
    	var hashInt big.Int
    	data := pow.prepareData(pow.block.Nonce)
    	hash := sha256.Sum256(data)
    	hashInt.SetBytes(hash[:])
    	isValid := hashInt.Cmp(pow.target)==-1
    	return isValid
    }

utils.go

    package main

    import (
    	"bytes"
    	"encoding/binary"
    	"log"
    )

    // Convert an int64 to a byte array
    func IntToHex(num int64) []byte{
    	buff := new(bytes.Buffer)
    	err := binary.Write(buff,binary.BigEndian,num)
    	if err!=nil{
    		log.Panic(err)
    	}
    	return buff.Bytes()
    }
