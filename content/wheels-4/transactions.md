# 交易

在区块链中，交易一旦被创建，就没有任何人能够再去修改或者删除它。

### UTXO模型

Bitcoin采用UTXO（Unspent Transaction output）模型作为其底层存储的数据结构，，也就是未被使用的交易输出。

某一个“账户”中的余额并不是由一个数字表示，而是由当前区块链网络中所有跟当前“账户”有关的UTXO组成的。

![](https://img.draveness.me/2018-04-05-balance-with-utxo-model.png)

上图中所有绿色的交易输出才是UTXO，红色的交易输出已经被当前“账户”使用了，所以在计算当前账户的余额时只考虑绿色的交易输出，
也就是UTXO。

### 比特币交易

一笔交易由一些输入（input）和输出（output）组合而来：

    type Transaction struct{
        ID []byte
        Vin []TXInput
        Vout    []TXOutput
    }

对于每一笔新的交易，它的输入会引用（reference）之前的一笔交易的输出，引用就是花费的意思。所谓引用之前的一个输出，也就是将
之前的一个输出包含在另一笔交易的输入当中，即iushi花费之前的交易输出。交易的输出，就是币实际存储的地方。

![](http://chuantu.biz/t6/323/1528083877x1822611335.jpg)

上图注意：

- 1.有一些输出并没有被关联到某个输入上
- 2.一笔交易的输入可以引用之前多笔交易的输出
- 3.一个输入必须引用一个输出

在现实生活中，我们将使用传统的概念，比如“钱”，“币”，“花费”，“发送”，“账户”等，但在比特币中，其实并没有这些概念，
那发生的交易是怎么一回事呢？其实仅仅是通过一个脚本（script）来锁定（lock）一些值（value），而这些值只能被锁定的人解锁（unlock）。

每一笔比特币交易都会创造输出，输出都会被区块链记录下来，给某个人发送比特币，那么底层发生了什么呢？实际上意味着创造新的UTXO
并注册到那个人的地址，可以让他使用。

### 交易输出

    type TXOutput struct{
        Value int
        ScriptPubKey    string
    }

输出主要包含两部分

- 1.一定量的比特币（value）
- 2.一个锁定脚本（ScriptPubKey）,要花这笔钱，必须要解锁该脚本

上面的比特币（value），实际上用一个数学难题对输出进行锁定，这个难题被存储在ScriptPubKey里面。

由于在这个版本的轮子中还没有地址的实现（address），所以目前我们让ScriptPubKey存储一个任意的字符串作为用户的钱包地址。

> 有了这个脚本语言，意味着比特币也可以作为一个智能合约平台。

关于输出，非常重要的一点就是，输出是不可再分的（indivisible）。怎么理解呢，就是无法仅引用它的某一部分，要么不用，如果要用，
必须一次性用完。当一个新的交易中引用了某个输出，那么这个输出必须被全部花费。如果它的值比需要的值大，就会产生一个找零，然后
找零会返还给发送方。

### 发送币

现在讲一下发送币的流程：

- 交易需要打包到一个块中，块被矿工挖出来
- 创建新的输出前，找到所有的未花费输出，并确保有足够的价值（value）
- 对于每个找到的输出，创建一个引用该输出的输入，接下来创建两个输出：
    - 一个接收者地址锁定，这是给其他地址实际转移的币。
    - 一个由发送者地址锁定。这是找零，只有当未花费输出超过新交易所需时产生。输出是不可再分的。
- 对所有未花费进行迭代，对它的值进行累加。当累加值大于或等于我们想要传送的价值时，就会停止并返回累加值，同时返回的还有
通过交易ID进行分组的输出索引。我们只需取出足够支付的币就够了。
- 发送币意味创建新的交易，挖矿将新块打包到链中。实际是将所有新的交易放到一个内存池中（mempool），然后当矿工准备挖出一个
新块时，就从内存池中取出所有交易，创建一个候选块。只有当包含这些交易的块被挖出，并添加到链以后，交易才算确认。

### 总结
这个版本的轮子，只是要实现交易的基础框架，一些关键特性是缺失的，是会在下几个版本的轮子中体现出来：

- 地址（address），还有基于私钥（private key）的真实地址
- 奖励（reward），挖矿是没有奖励的
- UTXO集，这个版本的轮子获取余额是需要扫描整个区块链，而当区块非常多的时候，扫描就太慢了。UTXO集就是加速交易相关的操作。
- 内存池（mempool），交易被打包到块之前，这些交易被存储在内存池里面。这个版本的轮子，一个块就一个交易。

### 交易输入

    type TXInput struct {
        Txid    []byte
        Vout    int
        ScriptSig   string
    }

一个输入引用了之前交易的一个输出：Txid是之前交易的ID，Vout是该输出在那笔交易中所有输出的索引，ScriptSig是一个脚本，提供了
可用来解锁输出结构ScriptPubKey字段的数据。怎么用呢，ScriptSig提供数据正确，输出就会被解锁，然后其值可以用来产生新的输出；
如果ScriptSig不正确，输出就无法被引用在输入中，无法使用这个输出。

概括一下：

- 输出，币存储的地方。
- 每个输出都带有一个解锁脚本，定义了解锁该输出的逻辑。
- 每笔新的交易，必须至少有一个输入和输出。
- 一个输入引用之前一笔交易的输出，并提供解锁数据（ScriptSig字段）。
- 该数据会被用在输出的解锁脚本中解锁输出，解锁完成后使用它的值去产生新的输出。

每一笔输入都是之前一笔交易的输出，第一笔交易只有输出，没有输入。

### 将交易保存到区块链

在这个版本的轮子中，每个块必须存储至少一笔交易。如果没有交易，就不产生新的块。

### 工作量证明

PoW算法必须要将存储到区块里面的交易考虑进去，从而保证区块链交易存储的一致性和可靠性。
通过哈希提供数据的唯一表示，这种做法通过仅仅一个哈希，可以识别一个块里面的所有交易。
为此，先获得每笔交易的哈希，然后将它们关联起来，最后获得一个连接后的组合哈希。

> 比特币使用了一种更加复杂的技术，将一个块里面包含的所有交易表示为一个Merkle tree，然后
在工作量证明系统中使用树的根哈希（root hash）。这个方法能够让我们快速检索一个块里面是否
包含了某笔交易，即只需root hash而无需下载所有交易即可完成判断。

### 未花费交易输出

找到所有的未花费交易输出（unspent transactions outputs，UTXO）。未花费（unspent）指的是这个输出
还没有被包含在任何交易的输入中，或者说没有被任何输入引用。

检查余额时，并不需要知道整个区块链上所有的UTXO，只需要关注那些我们能够解锁的那些UTXO。在这个版本的轮子中，使用
用户定义的地址来替代密钥。

所以解锁和上锁都是直接将script字段和unlockingData进行比较。

如果一个输出被一个地址锁定，并且恰好是我们要找的地址，那么这个输出就是我们想要的。不过在获取之前，需要检查是否被包含
在一个交易的输入中，也就是检查是否已经被花费了。

调过那些已经被包含在其他输入中的输出，检查完暑输出之后，将给定地址所有能够解锁输出的输入聚集起来。

### 调试命令

    go build -o blockchain_go
    ./blockchain_go -help
    ./blockchain_go createblockchain -address "mywalletaddress"
    ./blockchain_go printchain
    ./blockchain_go getbalance -address "mywalletaddress"
    ./blockchain_go send -to "yourwalletaddress" -from "mywalletaddress" -amount 3
    ./blockchain_go getbalance -address "yourwalletaddress"

### 项目代码拆分下面几个部分

block.go

    package main

    import (
    	"bytes"
    	"encoding/gob"
    	"log"
    	"time"
    	"crypto/sha256"
    )

    type Block struct {
    	Timestamp	int64
    	Transactions	[]*Transaction
    	PrevBlockHash	[]byte
    	Hash	[]byte
    	Nonce	int
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
    func NewBlock(transactions []*Transaction,prevBlockHash []byte) *Block{
    	block := &Block{
    		Timestamp:time.Now().Unix(),
    		Transactions:transactions,
    		PrevBlockHash:prevBlockHash,
    		Hash:[]byte{},
    		Nonce:0}
    	pow := NewProofOfWork(block)
    	nonce,hash := pow.Run()
    	block.Hash = hash
    	block.Nonce = nonce
    	return block
    }
    func NewGenesisBlock(coinbase *Transaction) *Block{
    	return NewBlock([]*Transaction{coinbase},[]byte{})
    }
    // 计算区块里所有交易的哈希
    func (block *Block) HashTransactions() []byte{
    	var txHashes [][]byte
    	var txHash [32]byte
    	for _,tx := range block.Transactions{
    		txHashes = append(txHashes,tx.ID)
    	}
    	txHash = sha256.Sum256(bytes.Join(txHashes,[]byte{}))
    	return txHash[:]
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

    const	targetBits = 24
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
    			pow.block.HashTransactions(),
    			IntToHex(pow.block.Timestamp),
    			IntToHex(int64(targetBits)),
    			IntToHex(int64(nonce)),},
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
    		hashInt.SetBytes(hash[:])
    		if hashInt.Cmp(pow.target)==-1{
    			fmt.Printf("\r%x", hash)
    			break
    		} else {
    			nonce++
    		}
    	}
    	fmt.Print("\n\n")
    	return nonce, hash[:]
    }
    func (pow *ProofOfWork) Validate() bool{
    	var hashInt big.Int
    	data := pow.prepareData(pow.block.Nonce)
    	hash := sha256.Sum256(data)
    	hashInt.SetBytes(hash[:])
    	isValid := hashInt.Cmp(pow.target) == -1
    	return isValid
    }

utils.go

    package main

    import (
    	"bytes"
    	"encoding/binary"
    	"log"
    )

    func IntToHex(num int64) []byte {
    	buff := new(bytes.Buffer)
    	err := binary.Write(buff, binary.BigEndian, num)
    	if err != nil {
    		log.Panic(err)
    	}
    	return buff.Bytes()
    }

transaction.go

    package main

    import (
    	"bytes"
    	"encoding/gob"
    	"log"
    	"crypto/sha256"
    	"fmt"
    	"encoding/hex"
    )

    const subsidy = 10 //是挖出新块的奖励金

    type Transaction struct {
    	ID	[]byte
    	Vin	[]TXInput
    	Vout	[]TXOutput
    }

    type TXInput struct {
    	Txid	[]byte  	// 一个交易输入引用之前一笔交易的一个输出
    	Vout	int			//一笔交易可能有多个输出，Vout为输出的索引
    	ScriptSig	string	//提供解锁输出Txid:Vout的数据
    }

    type TXOutput struct {
    	Value	int				//有多少币
    	ScriptPubKey	string	//对输出进行锁定
    }

    func (tx Transaction) IsCoinbase() bool {
    	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
    }
    func (tx *Transaction) SetID(){
    	var encoded bytes.Buffer
    	var hash [32]byte
    	encode := gob.NewEncoder(&encoded)
    	err := encode.Encode(tx)
    	if err != nil {
    		log.Panic(err)
    	}
    	hash = sha256.Sum256(encoded.Bytes())
    	tx.ID = hash[:]
    }
    func (in *TXInput) CanUnlockOutputWith(unlockingData string) bool{
    	return in.ScriptSig == unlockingData
    }
    func (out *TXOutput) CanBeUnlockedWith(unlockingData string) bool{
    	return out.ScriptPubKey == unlockingData
    }
    // NewCoinbaseTX 构建coinbase交易，没有输入，只有一个输出
    func NewCoinbaseTX(to,data string) *Transaction{
    	if data == "" {
    		data = fmt.Sprintf("Reward to '%s'", to)
    	}
    	txin := TXInput{[]byte{},-1,data}
    	txout := TXOutput{subsidy,to}
    	tx := Transaction{nil,[]TXInput{txin},[]TXOutput{txout}}
    	tx.SetID()
    	return &tx
    }
    // NewUTXOTransaction 创建一笔新的交易
    func NewUTXOTransaction(from,to string,amount int,blockchain *Blockchain) *Transaction{
    	var inputs []TXInput
    	var outputs []TXOutput
    	// 找到足够的未花费输出
    	acc,validOutputs := blockchain.FindSpendableOutputs(from,amount)
    	if acc<amount{
    		log.Panic("ERROR:Not enough funds")
    	}
    	for txid,outs := range validOutputs {
    		txID,err := hex.DecodeString(txid)
    		if err!=nil{
    			log.Panic(err)
    		}
    		for _,out := range outs{
    			input := TXInput{txID,out,from}
    			inputs = append(inputs,input)
    		}
    	}
    	outputs = append(outputs,TXOutput{amount,to})
    	// 如果UTXO总数超过所需，则产生找零
    	if acc>amount{
    		outputs = append(outputs,TXOutput{acc-amount,from})
    	}
    	tx := Transaction{nil,inputs,outputs}
    	tx.SetID()
    	return &tx
    }

blockchain.go

    package main

    import (
    	"github.com/boltdb/bolt"
    	"log"
    	"os"
    	"fmt"
    	"encoding/hex"
    )

    const dbFile = "blockchain.db"
    const blocksBucket = "blocks"
    const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

    type Blockchain struct {
    	tip []byte
    	db *bolt.DB
    }
    type BlockchainIterator struct {
    	currentHash	[]byte
    	db	*bolt.DB
    }
    func (blockchain *Blockchain) MineBlock(transactions []*Transaction){
    	var lastHash []byte
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
    }
    func (blockchain *Blockchain) Iterator() *BlockchainIterator{
    	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
    	return blockchainiterator
    }
    func (i *BlockchainIterator) Next() *Block {
    	var block *Block
    	err := i.db.View(func(tx *bolt.Tx) error {
    		bucket := tx.Bucket([]byte(blocksBucket))
    		encodedBlock := bucket.Get(i.currentHash)
    		block = DeserializeBlock(encodedBlock)
    		return nil
    	})
    	if err != nil {
    		log.Panic(err)
    	}
    	i.currentHash = block.PrevBlockHash
    	return block
    }
    func dbExists() bool{
    	if _,err := os.Stat(dbFile);os.IsNotExist(err){
    		return  false
    	}
    	return true
    }
    // 创建一个有创世块的新链
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
    // 创建一个新的区块链数据库，address用来接收挖出创世块的奖励
    func CreateBlockchain(address string) *Blockchain{
    	if dbExists(){
    		fmt.Println("Blockchain already exists.")
    		os.Exit(1)
    	}
    	var tip []byte
    	db,err := bolt.Open(dbFile,0600,nil)
    	if err != nil{
    		log.Panic(err)
    	}
    	err = db.Update(func(tx *bolt.Tx) error {
    		cbtx := NewCoinbaseTX(address,genesisCoinbaseData)
    		genesis := NewGenesisBlock(cbtx)
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
    // 找到未花费输出的交易
    func (blockchain *Blockchain) FindUnspentTransactions(address string) []Transaction{
    	var unspentTXs []Transaction
    	spentTXOs := make(map[string][]int)
    	blockchainiterator := blockchain.Iterator()
    	for {
    		block := blockchainiterator.Next()
    		for _,tx := range block.Transactions{
    			txID := hex.EncodeToString(tx.ID)
    		Outputs:
    			for outIdx,out := range tx.Vout{
    				if spentTXOs[txID] != nil { // 如果交易输出被花费了
    					for _,spentOut := range spentTXOs[txID]{
    						if spentOut == outIdx{
    							continue Outputs
    						}
    					}
    				}
    				// 如果该交易输出可以被解锁，即可倍花费
    				if out.CanBeUnlockedWith(address){
    					unspentTXs = append(unspentTXs,*tx)
    				}
    			}
    			if tx.IsCoinbase() == false{
    				for _,in := range tx.Vin{
    					if in.CanUnlockOutputWith(address){
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
    func (blockchain *Blockchain) FindUTXO(address string) []TXOutput {
    	var UTXOs []TXOutput
    	unspentTransactions := blockchain.FindUnspentTransactions(address)
    	for _,tx := range unspentTransactions{
    		for _,out := range tx.Vout{
    			if out.CanBeUnlockedWith(address){
    				UTXOs = append(UTXOs,out)
    			}
    		}
    	}
    	return UTXOs
    }
    // 从address中找到至少amount的UTXO
    func (blockchain *Blockchain) FindSpendableOutputs(address string,amount int) (int,map[string][]int){
    	unspentOutputs := make(map[string][]int)
    	unspentTXs := blockchain.FindUnspentTransactions(address)
    	accumulated := 0
    Work:
    	for _,tx := range unspentTXs{
    		txID := hex.EncodeToString(tx.ID)
    		for outIdx,out := range tx.Vout{
    			if out.CanBeUnlockedWith(address) && accumulated<amount{
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

cli.go

    package main

    import (
    	"fmt"
    	"os"
    	"strconv"
    	"flag"
    	"log"
    )

    type CLI struct {}

    func (cli *CLI) createBlockchain(address string){
    	blockchain := CreateBlockchain(address)
    	blockchain.db.Close()
    	fmt.Println("Done!")
    }
    func (cli *CLI) getBalance(address string){
    	blockchain := NewBlockchain(address)
    	defer blockchain.db.Close()
    	balance := 0
    	UTXOs := blockchain.FindUTXO(address)
    	for _,out := range UTXOs{
    		balance += out.Value
    	}
    	fmt.Printf("Balance of '%s': %d\n", address, balance)
    }
    func (cli *CLI) printUsage(){
    	fmt.Println("Usage:")
    	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
    	fmt.Println("  createblockchain -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
    	fmt.Println("  printchain - Print all the blocks of the blockchain")
    	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
    }
    func (cli *CLI) validateArgs(){
    	if len(os.Args)<2{
    		cli.printUsage()
    		os.Exit(1)
    	}
    }
    func (cli *CLI) printChain(){
    	blockchain := NewBlockchain("")
    	defer blockchain.db.Close()
    	blockchainiterator := blockchain.Iterator()
    	for {
    		block := blockchainiterator.Next()
    		fmt.Printf("Prev hash: %x\n", block.PrevBlockHash)
    		fmt.Printf("Hash: %x\n", block.Hash)
    		pow := NewProofOfWork(block)
    		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
    		fmt.Println()
    		if len(block.PrevBlockHash)==0{
    			break
    		}
    	}
    }
    func (cli *CLI) send(from,to string,amount int){
    	blockchain := NewBlockchain(from)
    	defer blockchain.db.Close()
    	tx := NewUTXOTransaction(from,to,amount,blockchain)
    	blockchain.MineBlock([]*Transaction{tx})
    	fmt.Println("Success!")
    }
    func (cli *CLI) Run(){
    	cli.validateArgs()
    	getBalanceCmd := flag.NewFlagSet("getbalance",flag.ExitOnError)
    	createBlockchainCmd := flag.NewFlagSet("createblockchain",flag.ExitOnError)
    	sendCmd := flag.NewFlagSet("send",flag.ExitOnError)
    	printChainCmd := flag.NewFlagSet("printchain",flag.ExitOnError)
    	getBalanceAddress := getBalanceCmd.String("address","","The address to get balance for")
    	createBlockchainAddress := createBlockchainCmd.String("address","","The address to send genesis block reward to")
    	sendFrom := sendCmd.String("from","","Source wallet address")
    	sendTo := sendCmd.String("to","","Destination wallet address")
    	sendAmount := sendCmd.Int("amount",0,"Amount to send")
    	switch os.Args[1] {
    	case "getbalance":
    		err := getBalanceCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "createblockchain":
    		err := createBlockchainCmd.Parse(os.Args[2:])
    		if err != nil{
    			log.Panic(err)
    		}
    	case "send":
    		err := sendCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	case "printchain":
    		err := printChainCmd.Parse(os.Args[2:])
    		if err != nil {
    			log.Panic(err)
    		}
    	default:
    		cli.printUsage()
    		os.Exit(1)
    	}
    	if getBalanceCmd.Parsed(){
    		if *getBalanceAddress == ""{
    			getBalanceCmd.Usage()
    			os.Exit(1)
    		}
    		cli.getBalance(*getBalanceAddress)
    	}
    	if createBlockchainCmd.Parsed(){
    		if *createBlockchainAddress==""{
    			createBlockchainCmd.Usage()
    			os.Exit(1)
    		}
    		cli.createBlockchain(*createBlockchainAddress)
    	}
    	if printChainCmd.Parsed(){
    		cli.printChain()
    	}
    	if sendCmd.Parsed(){
    		if *sendFrom=="" || *sendTo=="" || *sendAmount<=0{
    			sendCmd.Usage()
    			os.Exit(1)
    		}
    		cli.send(*sendFrom,*sendTo,*sendAmount)
    	}
    }

main.go

    package main

    func main()  {
    	cli := CLI{}
    	cli.Run()
    }

















