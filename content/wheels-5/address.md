# 地址

在上一个版本的轮子中，地址是自己随意命名的，这个版本就需要实现一个跟比特币一样的真实地址。

### 比特币地址

比特币中，身份（identity）是一对（或者多对）保存在你的电脑的公钥和私钥。

### 公钥加密

私钥和公钥只不过是随机的字节序列，无法在屏幕上打印，人类无法通过肉眼读取。比特币使用了一个转换算法，将公钥转化为一个
人类可读的字符串（地址）。

> 比特币钱包可以做一个助记符的功能，这样的助记符可以用来替代私钥，并且可用来生成私钥。BIP-039实现了这个机制。

### 数字签名

数字签名（digital signature），算法可以保证：

- 当数据从发送方传送到接收方时，数据不会被修改
- 数据由某一个确定的发送方创建
- 发送方无法否认发送过数据这一事实

在数据上签名，需要一个私钥，而别人验证这个签名的时候需要用到你的公钥。比如著名画家的一幅画，著名画家名字
就好比是公钥，画家的印章好比私钥，用私钥签名，用著名画家名字（公钥）就能结合印的章（签名）进行验证。

为了对数据进行签名，需要：

- 要签名的数据
- 私钥

生成的签名会被存储在交易输入中，为了对一个签名进行验证，需要：

- 被签名的数据
- 签名
- 公钥

在比特币中，每一笔交易输入都会由创建交易的人签名。在被放入到一个块之前，必须要对每一笔交易进行验证。

- 1.检查交易输入有权使用来自之前交易的输出
- 2.检查交易签名是正确的

对数据进行签名和对签名进行验证的过程如下：

![](http://chuantu.biz/t6/324/1528177426x1822611335.jpg)

有了签名和验证，接下来让我们看看整个交易的声明周期：

- 1.创世块包含一个coinbase交易。在coinbase交易中，没有输入，不需签名。coinbase交易的输出包含一个哈希过的公钥。
- 2.发送币时，创建一笔交易。这笔交易的输入会引用之前交易的输出。每个输入会存储一个公钥和整个交易的一个签名。
- 3.进行广播，其他节点接收到交易后对该交易进行验证。在一个输入中，公钥哈希与所使用的输出哈希相匹配（保证发送方只能花费自己的币）；签名是正确的（保证了交易由币的拥有者创建）。
- 4.矿工准备挖一个新块时，会将交易放到块中，然后开始挖矿。
- 5.新的块挖出，进行全网广播，包含了此交易的块已经写入了区块链。
- 6.块被写入区块链后，交易完成，输出就可以在新的交易中被引用。

### 椭圆曲线加密

比特币使用椭圆曲线来产生私钥。总之就是生成的私钥是唯一的，别人碰撞不到你的私钥。

#### Base58

比特币使用Base58算法将公钥转换成人类可读的形式。

### 测试命令

    go build -o blockchain_go
    ./blockchain_go -help
    ./blockchain_go createwallet
    ./blockchain_go listaddresses
    ./blockchain_go getbalance -address 1Pqz1u4JnuBWuuCSPu6jZZXqbbrTNmohgh
    ./blockchain_go createblockchain -address 1Pqz1u4JnuBWuuCSPu6jZZXqbbrTNmohgh
    ./blockchain_go printchain
    ./blockchain_go send -from 1Pqz1u4JnuBWuuCSPu6jZZXqbbrTNmohgh -to 13GnPK9yGzoAHpTs5CqbsR3DDzLEUeGa7V -amount 1

### 项目代码

base58.go

    package main

    import (
    	"math/big"
    	"bytes"
    )

    var b58Alphabet = []byte("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")
    // Base58Encode encodes a byte array to Base58
    func Base58Encode(input []byte) []byte{
    	var result []byte
    	x := big.NewInt(0).SetBytes(input)
    	base := big.NewInt(int64(len(b58Alphabet)))
    	zero :=  big.NewInt(0)
    	mod := &big.Int{}
    	for x.Cmp(zero)!=0{
    		x.DivMod(x,base,mod)
    		result = append(result,b58Alphabet[mod.Int64()])
    	}
    	ReverseBytes(result)
    	for b:=range input{
    		if b==0x00{
    			result = append([]byte{b58Alphabet[0]},result...)
    		}else{
    			break
    		}
    	}
    	return result
    }
    // Base58Decode decodes Base58-encoded data
    func Base58Decode(input []byte) []byte {
    	result := big.NewInt(0)
    	zeroBytes := 0
    	for b := range input {
    		if b == 0x00 {
    			zeroBytes++
    		}
    	}
    	payload := input[zeroBytes:]
    	for _, b := range payload {
    		charIndex := bytes.IndexByte(b58Alphabet, b)
    		result.Mul(result, big.NewInt(58))
    		result.Add(result, big.NewInt(int64(charIndex)))
    	}
    	decoded := result.Bytes()
    	decoded = append(bytes.Repeat([]byte{byte(0x00)}, zeroBytes), decoded...)
    	return decoded
    }

block.go

    package main

    import (
    	"time"
    	"crypto/sha256"
    	"bytes"
    	"encoding/gob"
    	"log"
    )

    type Block struct {
    	Timestamp	int64
    	Transactions	[]*Transaction
    	PrevBlockHash	[]byte
    	Hash	[]byte
    	Nonce	int
    }
    func NewBlock(transactions []*Transaction,prevBlockHash []byte) *Block{
    	block := &Block{time.Now().Unix(),transactions,prevBlockHash,[]byte{},0}
    	pow := NewProofOfWork(block)
    	nonce,hash := pow.Run()
    	block.Hash = hash[:]
    	block.Nonce = nonce
    	return block
    }
    func NewGenesisBlock(coinbase *Transaction) *Block{
    	return NewBlock([]*Transaction{coinbase},[]byte{})
    }
    // returns a hash of the transactions in the block
    func (block *Block) HashTransactions() []byte{
    	var txHashes [][]byte
    	var txHash [32]byte
    	for _,tx := range block.Transactions{
    		txHashes = append(txHashes,tx.Hash())
    	}
    	txHash = sha256.Sum256(bytes.Join(txHashes,[]byte{}))
    	return txHash[:]
    }
    func (block *Block) Serialize() []byte{
    	var result bytes.Buffer
    	encoder := gob.NewEncoder(&result)
    	err := encoder.Encode(block)
    	if err != nil {
    		log.Panic(err)
    	}
    	return result.Bytes()
    }
    func DeserializeBlock(d []byte) *Block{
    	var block Block
    	decoder := gob.NewDecoder(bytes.NewReader(d))
    	err := decoder.Decode(&block)
    	if err != nil {
    		log.Panic(err)
    	}
    	return &block
    }

blockchain.go

    package main

    import (
    	"github.com/boltdb/bolt"
    	"fmt"
    	"os"
    	"log"
    	"encoding/hex"
    	"bytes"
    	"github.com/pkg/errors"
    	"crypto/ecdsa"
    )

    const dbFile = "blockchain.db"
    const blocksBucket = "blocks"
    const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

    type Blockchain struct {
    	tip	[]byte
    	db	*bolt.DB
    }
    type BlockchainIterator struct {
    	currentHash	[]byte
    	db	*bolt.DB
    }
    func CreateBlockchain(address string) *Blockchain{
    	if dbExists(){
    		fmt.Println("Blockchain already exists.")
    		os.Exit(1)
    	}
    	var tip []byte
    	cbtx := NewCoinbaseTX(address,genesisCoinbaseData)
    	genesis := NewGenesisBlock(cbtx)
    	db,err := bolt.Open(dbFile,0600,nil)
    	if err != nil {
    		log.Panic(err)
    	}
    	err = db.Update(func(tx *bolt.Tx) error {
    		bucket,err := tx.CreateBucket([]byte(blocksBucket))
    		if err != nil {
    			log.Panic(err)
    		}
    		err = bucket.Put(genesis.Hash,genesis.Serialize())
    		if err != nil {
    			log.Panic(err)
    		}
    		err = bucket.Put([]byte("1"),genesis.Hash)
    		if err != nil {
    			log.Panic(err)
    		}
    		tip = genesis.Hash
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	blockchain := Blockchain{tip,db}
    	return &blockchain
    }
    func NewBlockchain(address string) *Blockchain{
    	if dbExists()==false{
    		fmt.Println("No existing blockchain found. Create one first.")
    		os.Exit(1)
    	}
    	var tip []byte
    	db,err := bolt.Open(dbFile,0600,nil)
    	if err != nil {
    		log.Panic(err)
    	}
    	err = db.Update(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		tip = bucket.Get([]byte("1"))
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	blockchain := Blockchain{tip,db}
    	return &blockchain
    }
    // finds and returns unspent outputs to reference in inputs
    func (blockchain *Blockchain) FindSpendableOutputs(pubKeyHash []byte,amount int) (int,map[string][]int){
    	unspentOutputs := make(map[string][]int)
    	unspentTXs := blockchain.FindUnspentTransactions(pubKeyHash)
    	accumulated := 0
    Work:
    	for _,tx := range unspentTXs{
    		txID := hex.EncodeToString(tx.ID)
    		for outIdx,out := range tx.Vout{
    			if out.IsLockedWithKey(pubKeyHash) && accumulated<amount{
    				accumulated += out.Value
    				unspentOutputs[txID] = append(unspentOutputs[txID],outIdx)
    				if accumulated>=amount{
    					break Work
    				}
    			}
    		}
    	}
    	return accumulated,unspentOutputs
    }
    // finds a transaction by its ID
    func (blockchain *Blockchain) FindTransaction(ID []byte) (Transaction,error){
    	blockchainiterator := blockchain.Iterator()
    	for {
    		block := blockchainiterator.Next()
    		for _,tx := range block.Transactions{
    			if bytes.Compare(tx.ID,ID)==0{
    				return *tx,nil
    			}
    		}
    		if len(block.PrevBlockHash)==0{
    			break
    		}
    	}
    	return Transaction{},errors.New("Transaction is not found")
    }
    // returns da list of transactions containing unspent outputs
    func (blockchain *Blockchain) FindUnspentTransactions(pubKeyHash []byte) []Transaction{
    	var unspentTXs []Transaction
    	spentTXOs := make(map[string][]int)
    	blockchainiterator := blockchain.Iterator()
    	for{
    		block := blockchainiterator.Next()
    		for _,tx := range block.Transactions{
    			txID := hex.EncodeToString(tx.ID)
    		Outputs:
    			for outIdx,out := range tx.Vout{
    				if spentTXOs[txID] != nil{
    					for _,spentOutIdx := range spentTXOs[txID]{
    						if spentOutIdx==outIdx{
    							continue Outputs
    						}
    					}
    				}
    				if out.IsLockedWithKey(pubKeyHash){
    					unspentTXs = append(unspentTXs,*tx)
    				}
    			}
    			if tx.IsCoinbase() == false{
    				for _,in := range tx.Vin{
    					if in.UsesKey(pubKeyHash){
    						inTxID := hex.EncodeToString(in.Txid)
    						spentTXOs[inTxID] = append(spentTXOs[inTxID],in.Vout)
    					}
    				}
    			}
    		}
    		if len(block.PrevBlockHash)==0{
    			break
    		}
    	}
    	return unspentTXs
    }
    // finds and returns all unspent transaction outputs
    func (blockchain *Blockchain) FindUTXO(pubKeyHash []byte) []TXOutput{
    	var UTXOs []TXOutput
    	unspentTransactions := blockchain.FindUnspentTransactions(pubKeyHash)
    	for _,tx := range unspentTransactions{
    		for _,out := range tx.Vout{
    			if out.IsLockedWithKey(pubKeyHash){
    				UTXOs = append(UTXOs,out)
    			}
    		}
    	}
    	return UTXOs
    }
    func (blockchain *Blockchain) Iterator() *BlockchainIterator{
    	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
    	return blockchainiterator
    }
    // mines a new block with the provided transactions
    func (blockchain *Blockchain) MineBlock(transactions []*Transaction){
    	var lastHash []byte
    	for _,tx := range transactions{
    		if blockchain.VerifyTransaction(tx) != true{
    			log.Panic("ERROR: Invalid transaction")
    		}
    	}
    	err := blockchain.db.View(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		lastHash = bucket.Get([]byte("1"))
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	newBlock := NewBlock(transactions,lastHash)
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
    	if err != nil {
    		log.Panic(err)
    	}
    }

    // signs inputs of a Transaction
    func (blockchain *Blockchain) SignTransaction(tx *Transaction,privKey ecdsa.PrivateKey){
    	prevTXs := make(map[string]Transaction)
    	for _,vin := range tx.Vin{
    		prevTX,err := blockchain.FindTransaction(vin.Txid)
    		if err != nil {
    			log.Panic(err)
    		}
    		prevTXs[hex.EncodeToString(prevTX.ID)]=prevTX
    	}
    	tx.Sign(privKey,prevTXs)
    }

    // verifies transaction input signatures
    func (blockchain *Blockchain) VerifyTransaction(tx *Transaction) bool{
    	prevTXs := make(map[string]Transaction)
    	for _,vin := range tx.Vin{
    		prevTX,err := blockchain.FindTransaction(vin.Txid)
    		if err != nil {
    			log.Panic(err)
    		}
    		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
    	}
    	return tx.Verify(prevTXs)
    }

    func (blockchainiterator *BlockchainIterator) Next() *Block{
    	var block *Block
    	err := blockchainiterator.db.View(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		encodedBlock := bucket.Get(blockchainiterator.currentHash)
    		block = DeserializeBlock(encodedBlock)
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	blockchainiterator.currentHash = block.PrevBlockHash
    	return block
    }
    func dbExists() bool{
    	if _,err := os.Stat(dbFile);os.IsNotExist(err){
    		return false
    	}
    	return true
    }

cli.go

    package main

    import (
    	"fmt"
    	"os"
    	"flag"
    	"log"
    	"strconv"
    )

    type CLI struct {}

    func (cli *CLI) printUsage(){
    	fmt.Println("Usage:")
    	fmt.Println("  createblockchain -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
    	fmt.Println("  createwallet - Generates a new key-pair and saves it into the wallet file")
    	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
    	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
    	fmt.Println("  printchain - Print all the blocks of the blockchain")
    	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
    }

    func (cli *CLI) validateArgs() {
    	if len(os.Args) < 2 {
    		cli.printUsage()
    		os.Exit(1)
    	}
    }

    func (cli *CLI) Run(){
    	cli.validateArgs()
    	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
    	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
    	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
    	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
    	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
    	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)

    	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
    	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
    	sendFrom := sendCmd.String("from", "", "Source wallet address")
    	sendTo := sendCmd.String("to", "", "Destination wallet address")
    	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
    	switch os.Args[1] {
    	case "getbalance":
    		err := getBalanceCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "createblockchain":
    		err := createBlockchainCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "createwallet":
    		err := createWalletCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "listaddresses":
    		err := listAddressesCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "printchain":
    		err := printChainCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "send":
    		err := sendCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	default:
    		cli.printUsage()
    		os.Exit(1)
    	}
    	if getBalanceCmd.Parsed() {
    		if *getBalanceAddress == "" {
    			getBalanceCmd.Usage()
    			os.Exit(1)
    		}
    		cli.getBalance(*getBalanceAddress)
    	}

    	if createBlockchainCmd.Parsed() {
    		if *createBlockchainAddress == "" {
    			createBlockchainCmd.Usage()
    			os.Exit(1)
    		}
    		cli.createBlockchain(*createBlockchainAddress)
    	}

    	if createWalletCmd.Parsed() {
    		cli.createWallet()
    	}

    	if listAddressesCmd.Parsed() {
    		cli.listAddresses()
    	}

    	if printChainCmd.Parsed() {
    		cli.printChain()
    	}

    	if sendCmd.Parsed() {
    		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
    			sendCmd.Usage()
    			os.Exit(1)
    		}

    		cli.send(*sendFrom, *sendTo, *sendAmount)
    	}
    }

    func (cli *CLI) createBlockchain(address string){
    	if !ValidateAddress(address){
    		log.Panic("ERROR: Address is not valid")
    	}
    	blockchain := CreateBlockchain(address)
    	blockchain.db.Close()
    	fmt.Println("Done!")
    }

    func (cli *CLI) createWallet(){
    	wallets,_ := NewWallets()
    	address := wallets.CreateWallet()
    	wallets.SaveToFile()
    	fmt.Printf("Your new address: %s\n", address)
    }

    func (cli *CLI) getBalance(address string){
    	if !ValidateAddress(address){
    		log.Panic("ERROR: Address is not valid")
    	}
    	blockchain := NewBlockchain(address)
    	defer blockchain.db.Close()
    	balance := 0
    	pubKeyHash := Base58Decode([]byte(address))
    	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-4]
    	UTXOs := blockchain.FindUTXO(pubKeyHash)
    	for _,out := range UTXOs{
    		balance += out.Value
    	}
    	fmt.Printf("Balance of '%s': %d\n", address, balance)
    }

    func (cli *CLI) listAddresses(){
    	wallets,err := NewWallets()
    	if err != nil {
    		log.Panic(err)
    	}
    	addresses := wallets.GetAddresses()
    	for _,address := range addresses{
    		fmt.Println(address)
    	}
    }

    func (cli *CLI) printChain(){
    	blockchain := NewBlockchain("")
    	defer blockchain.db.Close()
    	blockchainiterator := blockchain.Iterator()
    	for {
    		block := blockchainiterator.Next()
    		fmt.Printf("============ Block %x ============\n", block.Hash)
    		fmt.Printf("Prev. block: %x\n", block.PrevBlockHash)
    		pow := NewProofOfWork(block)
    		fmt.Printf("PoW: %s\n\n", strconv.FormatBool(pow.Validate()))
    		for _, tx := range block.Transactions {
    			fmt.Println(tx)
    		}
    		fmt.Printf("\n\n")
    		if len(block.PrevBlockHash) == 0 {
    			break
    		}
    	}
    }

    func (cli *CLI) send(from,to string,amount int){
    	if !ValidateAddress(from){
    		log.Panic("ERROR: Sender address is not valid")
    	}
    	if !ValidateAddress(to){
    		log.Panic("ERROR: Recipient address is not valid")
    	}
    	blockchain := NewBlockchain(from)
    	defer blockchain.db.Close()
    	tx := NewUTXOTransaction(from,to,amount,blockchain)
    	blockchain.MineBlock([]*Transaction{tx})
    	fmt.Println("Success!")
    }

main.go

    package main

    func main() {
    	cli := CLI{}
    	cli.Run()
    }

proofofwork.go

    package main

    import (
    	"math"
    	"math/big"
    	"fmt"
    	"bytes"
    	"crypto/sha256"
    )

    var (
    	maxNonce = math.MaxInt64
    )
    const targetBits = 16

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
    			pow.block.HashTransactions(),
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
    	fmt.Printf("Mining a new block")
    	for nonce<maxNonce{
    		data := pow.prepareData(nonce)
    		hash = sha256.Sum256(data)
    		fmt.Printf("\r%x", hash)
    		hashInt.SetBytes(hash[:])
    		if hashInt.Cmp(pow.target)==-1{
    			break
    		}else {
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

transaction.go

    package main

    import (
    	"bytes"
    	"encoding/gob"
    	"log"
    	"crypto/sha256"
    	"crypto/ecdsa"
    	"encoding/hex"
    	"crypto/rand"
    	"fmt"
    	"strings"
    	"crypto/elliptic"
    	"math/big"
    )

    const subsidy = 10
    type Transaction struct {
    	ID	[]byte
    	Vin	[]TXInput
    	Vout	[]TXOutput
    }
    type TXInput struct {
    	Txid	[]byte
    	Vout	int
    	Signature	[]byte
    	PubKey	[]byte
    }
    // UsesKey checks whether the address initiated the transaction
    func (in *TXInput)	UsesKey(pubKeyHash	[]byte) bool{
    	lockingHash	:= HashPubKey(in.PubKey)
    	return bytes.Compare(lockingHash,pubKeyHash)==0
    }
    type TXOutput struct {
    	Value	int
    	PubKeyHash	[]byte
    }
    // signs the output
    func (out *TXOutput)	Lock(address []byte){
    	pubKeyHash	:= Base58Decode(address)
    	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-4]
    	out.PubKeyHash = pubKeyHash
    }
    // checks if the output can be used by the owner of the pubkey
    func (out *TXOutput) IsLockedWithKey(pubKeyHash	 []byte) bool{
    	return bytes.Compare(out.PubKeyHash,pubKeyHash)==0
    }
    func NewTXOutput(value int,address string) *TXOutput{
    	txo := &TXOutput{value,nil}
    	txo.Lock([]byte(address))
    	return txo
    }
    // checks whether the transaction is coinbase
    func (tx Transaction) IsCoinbase() bool{
    	return len(tx.Vin)==1&&len(tx.Vin[0].Txid)==0&&tx.Vin[0].Vout==-1
    }
    func (tx Transaction) Serialize() []byte{
    	var encoded bytes.Buffer
    	enc := gob.NewEncoder(&encoded)
    	err := enc.Encode(tx)
    	if err != nil {
    		log.Panic(err)
    	}
    	return encoded.Bytes()
    }
    // returns the hash of the Transaction
    func (tx *Transaction) Hash() []byte{
    	var hash [32]byte
    	txCopy := *tx
    	txCopy.ID = []byte{}
    	hash = sha256.Sum256(txCopy.Serialize())
    	return hash[:]
    }
    // signs each input of a Transaction
    func (tx *Transaction) Sign(privKey ecdsa.PrivateKey,prevTXs map[string]Transaction){
    	if tx.IsCoinbase(){
    		return
    	}
    	for _,vin:= range tx.Vin{
    		if prevTXs[hex.EncodeToString(vin.Txid)].ID==nil{
    			log.Panic("ERROR: Previous transaction is not correct")
    		}
    	}
    	txCopy := tx.TrimmedCopy()
    	for inID,vin := range txCopy.Vin{
    		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
    		txCopy.Vin[inID].Signature = nil
    		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash
    		txCopy.ID = txCopy.Hash()
    		txCopy.Vin[inID].PubKey = nil
    		r,s,err := ecdsa.Sign(rand.Reader,&privKey,txCopy.ID)
    		if err != nil {
    			log.Panic(err)
    		}
    		signature := append(r.Bytes(),s.Bytes()...)
    		tx.Vin[inID].Signature = signature
    	}
    }
    // creates a trimmed copy of Transaction to be used in signing
    func (tx *Transaction) TrimmedCopy() Transaction{
    	var inputs []TXInput
    	var outputs []TXOutput
    	for _,vin := range tx.Vin{
    		inputs = append(inputs,TXInput{vin.Txid,vin.Vout,nil,nil})
    	}
    	for _,vout := range tx.Vout{
    		outputs = append(outputs,TXOutput{vout.Value,vout.PubKeyHash})
    	}
    	txCopy := Transaction{tx.ID,inputs,outputs}
    	return txCopy
    }
    // return a human-readable representation of a transaction
    func (tx Transaction) String() string{
    	var lines []string
    	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
    	for i,input := range tx.Vin{
    		lines = append(lines, fmt.Sprintf("     Input %d:", i))
    		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
    		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
    		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
    		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
    	}
    	for i,output := range tx.Vout{
    		lines = append(lines, fmt.Sprintf("     Output %d:", i))
    		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
    		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
    	}
    	return strings.Join(lines,"\n")
    }
    // verifies signatures of Transaction inputs
    func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool{
    	if tx.IsCoinbase(){
    		return true
    	}
    	for _,vin := range tx.Vin{
    		if prevTXs[hex.EncodeToString(vin.Txid)].ID==nil{
    			log.Panic("ERROR: Previous transaction is not correct")
    		}
    	}
    	txCopy := tx.TrimmedCopy()
    	curve := elliptic.P256()
    	for inID,vin := range tx.Vin{
    		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
    		txCopy.Vin[inID].Signature = nil
    		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash
    		txCopy.ID = txCopy.Hash()
    		txCopy.Vin[inID].PubKey = nil
    		r := big.Int{}
    		s := big.Int{}
    		sigLen := len(vin.Signature)
    		r.SetBytes(vin.Signature[:(sigLen/2)])
    		s.SetBytes(vin.Signature[(sigLen/2):])
    		x := big.Int{}
    		y := big.Int{}
    		keyLen := len(vin.PubKey)
    		x.SetBytes(vin.PubKey[:(keyLen/2)])
    		y.SetBytes(vin.PubKey[(keyLen/2):])
    		rawPubKey := ecdsa.PublicKey{curve,&x,&y}
    		if ecdsa.Verify(&rawPubKey,txCopy.ID,&r,&s)==false{
    			return false
    		}
    	}
    	return true
    }
    func NewCoinbaseTX(to,data string) *Transaction{
    	if data == ""{
    		data = fmt.Sprintf("Reward to '%s'", to)
    	}
    	txin := TXInput{[]byte{},-1,nil,[]byte(data)}
    	txout := NewTXOutput(subsidy,to)
    	tx := Transaction{nil,[]TXInput{txin},[]TXOutput{*txout}}
    	tx.ID = tx.Hash()
    	return &tx
    }

    // creates a new transaction
    func NewUTXOTransaction(from,to string,amount int,blockchain *Blockchain) *Transaction {
    	var inputs []TXInput
    	var outputs []TXOutput
    	wallets,err := NewWallets()
    	if err != nil {
    		log.Panic(err)
    	}
    	wallet := wallets.GetWallet(from)
    	pubKeyHash := HashPubKey(wallet.PublicKey)
    	acc ,validOutputs := blockchain.FindSpendableOutputs(pubKeyHash,amount)
    	if acc < amount{
    		log.Panic("ERROR:Not enough funds")
    	}
    	// build a list of inputs
    	for txid,outs := range validOutputs{
    		txID,err := hex.DecodeString(txid)
    		if err != nil {
    			log.Panic(err)
    		}
    		for _,out := range outs{
    			input := TXInput{txID,out,nil,wallet.PublicKey}
    			inputs = append(inputs,input)
    		}
    	}
    	// build a list of outputs
    	outputs = append(outputs,*NewTXOutput(amount,to))
    	if acc > amount{
    		outputs = append(outputs,*NewTXOutput(acc-amount,from))
    	}
    	tx := Transaction{nil,inputs,outputs}
    	tx.ID = tx.Hash()
    	blockchain.SignTransaction(&tx,wallet.PrivateKey)
    	return &tx
    }

utils.go

    package main

    import (
    	"bytes"
    	"encoding/binary"
    	"log"
    )

    // IntToHex converts an int64 to a byte array
    func IntToHex(num int64) []byte {
    	buff := new(bytes.Buffer)
    	err := binary.Write(buff, binary.BigEndian, num)
    	if err != nil {
    		log.Panic(err)
    	}
    	return buff.Bytes()
    }
    // ReverseBytes reverses a byte array
    func ReverseBytes(data []byte) {
    	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
    		data[i], data[j] = data[j], data[i]
    	}
    }

wallet.go

    package main

    import (
    	"crypto/ecdsa"
    	"crypto/elliptic"
    	"crypto/rand"
    	"log"
    	"crypto/sha256"
    	"golang.org/x/crypto/ripemd160"
    	"bytes"
    	"os"
    	"io/ioutil"
    	"encoding/gob"
    	"fmt"
    )

    const version = byte(0x00)
    const walletFile = "wallet.dat"
    const addressChecksumLen = 4

    // Wallet stores privatte and public keys
    type Wallet struct {
    	PrivateKey	ecdsa.PrivateKey
    	PublicKey	[]byte
    }
    type Wallets struct {
    	Wallets map[string]*Wallet
    }

    // creates Wallets and fills it form a file f it exists
    func NewWallets() (*Wallets,error){
    	wallets := Wallets{}
    	wallets.Wallets = make(map[string]*Wallet)
    	err := wallets.LoadFromFile()
    	return &wallets,err
    }

    // adds a Wallet to Wallets
    func (ws *Wallets) CreateWallet() string{
    	wallet := NewWallet()
    	address := fmt.Sprintf("%s",wallet.GetAddress())
    	ws.Wallets[address] = wallet
    	return address
    }

    // return s an array of addresses stored in the wallet file
    func (ws *Wallets) GetAddresses() []string{
    	var addresses []string
    	for address := range ws.Wallets{
    		addresses = append(addresses,address)
    	}
    	return addresses
    }

    // returns a Wallet by its address
    func (ws Wallets) GetWallet(address string) Wallet{
    	return *ws.Wallets[address]
    }



    // loads wallets from the file
    func (ws *Wallets) LoadFromFile() error{
    	if _,err := os.Stat(walletFile);os.IsNotExist(err){
    		return err
    	}
    	fileContent,err := ioutil.ReadFile(walletFile)
    	if err != nil {
    		log.Panic(err)
    	}
    	var wallets Wallets
    	gob.Register(elliptic.P256())
    	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
    	err = decoder.Decode(&wallets)
    	if err != nil {
    		log.Panic(err)
    	}
    	ws.Wallets = wallets.Wallets
    	return nil
    }

    // saves wallets to a file
    func (ws Wallets) SaveToFile(){
    	var content bytes.Buffer
    	gob.Register(elliptic.P256())
    	encoder := gob.NewEncoder(&content)
    	err := encoder.Encode(ws)
    	if err != nil {
    		log.Panic(err)
    	}
    	err = ioutil.WriteFile(walletFile,content.Bytes(),0644)
    	if err != nil {
    		log.Panic(err)
    	}
    }

    func newKeyPair() (ecdsa.PrivateKey,[]byte){
    	curve := elliptic.P256()
    	private,err := ecdsa.GenerateKey(curve,rand.Reader)
    	if err!=nil{
    		log.Panic(err)
    	}
    	pubKey := append(private.PublicKey.X.Bytes(),private.PublicKey.Y.Bytes()...)
    	return *private,pubKey
    }
    func NewWallet() *Wallet{
    	private,public := newKeyPair()
    	wallet := Wallet{private,public}
    	return &wallet
    }
    func HashPubKey(pubKey []byte) []byte{
    	publicSHA256 := sha256.Sum256(pubKey)
    	RIPEMD160Hasher := ripemd160.New()
    	_,err := RIPEMD160Hasher.Write(publicSHA256[:])
    	if err != nil {
    		log.Panic(err)
    	}
    	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)
    	return publicRIPEMD160
    }
    // Checksum generates a checksum for a public key
    func checksum(payload []byte) []byte{
    	firstSHA := sha256.Sum256(payload)
    	secondSHA := sha256.Sum256(firstSHA[:])
    	return secondSHA[:addressChecksumLen]
    }
    func (wallet Wallet) GetAddress() []byte{
    	pubKeyHash := HashPubKey(wallet.PublicKey)
    	versionedPayload := append([]byte{version},pubKeyHash...)
    	checksum := checksum(versionedPayload)
    	fullPayload := append(versionedPayload,checksum...)
    	address := Base58Encode(fullPayload)
    	return address
    }
    // validateAddress check if address if valid
    func ValidateAddress(address string) bool{
    	pubKeyHash := Base58Decode([]byte(address))
    	actualChecksum := pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
    	version := pubKeyHash[0]
    	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-addressChecksumLen]
    	targetChecksum := checksum(append([]byte{version},pubKeyHash...))
    	return bytes.Compare(actualChecksum,targetChecksum)==0
    }