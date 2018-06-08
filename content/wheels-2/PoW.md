### 工作量证明（Proof-of-Work）

要往区块链中写入一个新的区块，需要获得大部分节点的认同，也就是要取得共识，
有一种方式就是付出巨大的代价，完成一些非常困难的工作，这样获得记账（写入区块）
的权力，这样就是通过工作量证明的方式达成共识。

通过完成这样一系列困难的工作，保证区块链的安全和一致，完成这样的工作就会获得奖励（也就是挖矿）。
当然完成困难工作是需要自己证明这一点的，这就是（Proof-of-Work）。

### 哈希计算

在区块链中，哈希被用来保证一个块的一致性。哈希算法的输入数据包含了另一个块的哈希，因此使得不太可能
去修改链中的一个块，因此如果想作弊修改一个块的哈希，那么必须重新计算这个块以及后面所有快的哈希。
也就是说需要强行将别人的工作量证明推翻，也就是我们所说的51%攻击。

### Hashcash

Hashcash，一个最初用来防止垃圾邮件的工作量证明算法。可以分解为以下步骤：

- 取一些公开的数据，比如比特币的区块头
- 给这公开数据添加一个计算器，默认从0开始
- 将数据和计算器组合，获得一个hash
- 检查哈希是否符合一定的条件：
    - 如果符合条件，结束
    - 如果不符合，增加计算器，重复3-4步

### 实现

首先，定义挖矿的难度值

    const targetBits = 24
    // 24指的是计算出来的哈希前24位必须是0，如果用16进制，前6位必须是0.
    // 当然24是固定的，实际是可以调整的

然后，构造ProofOfWork结构

    type ProofOfWork struct {
        block *Block
        target *big.Int
    }
    func NewProofOfWork(block *Block) *ProofOfWork {
        target := big.NewInt(1)
        target.Lsh(target,uint(256-targetBits))
        pow := &ProofOfWork{block,target}
        return pow
    }
    // ProofOfWork结构，存储了指向一个块（block）和一个目标（target）的指针。目标也就是必要条件，使用一个大整数，
    // 将哈希和目标进行比较：先把哈希转成一个大整数，然后检测它是否小于目标
    // 将bit.Int初始化为1，然后左移256-targetBits位

main.go

    package main

    import (
    	"time"
    	"math/big"
    	"crypto/sha256"
    	"fmt"
    	"math"
    	"bytes"
    	"encoding/binary"
    	"log"
    	"strconv"
    )

    // 难度值，表示哈希的前24位必须是0，16进制表示前6位是0
    const targetBits = 24
    const maxNonce  = math.MaxInt64

    type Block struct {
    	Timestamp	int64
    	PrevBlockHash	[]byte
    	Hash	[]byte
    	Data	[]byte
    	Nonce	int
    }
    type BlockChain struct {
    	blocks []*Block
    }

    type ProofOfWork struct {
    	block *Block
    	target	*big.Int
    }

    // 讲一个int64转化成一个字节数组（byte array）
    func IntToHex(num int64) []byte{
    	buff:=new(bytes.Buffer)
    	err:=binary.Write(buff,binary.BigEndian,num)
    	if err!=nil{
    		log.Panic(err)
    	}
    	return buff.Bytes()
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

    // PoW的核心就是寻找有效哈希
    func (pow *ProofOfWork) Run() (int,[]byte){
    	var hashInt big.Int
    	var hash [32]byte
    	nonce := 0
    	fmt.Printf("Mining the block containing \"%s\"\n", pow.block.Data)
    	for nonce<maxNonce{
    		data := pow.prepareData(nonce)
    		hash = sha256.Sum256(data)
    		hashInt.SetBytes(hash[:])
    		if hashInt.Cmp(pow.target) == -1 {
    			fmt.Printf("\r%x", hash)
    			break
    		}else{
    			nonce++
    		}
    	}
    	fmt.Print("\n\n")
    	return nonce,hash[:]
    }

    // 证明工作量，只要哈希小于目标就是有效工作量
    func (pow *ProofOfWork) Validate() bool {
    	var hashInt big.Int
    	data := pow.prepareData(pow.block.Nonce)
    	hash := sha256.Sum256(data)
    	hashInt.SetBytes(hash[:])
    	isValid := hashInt.Cmp(pow.target)==1
    	return isValid
    }

    // 创建新区块要用PoW找到有效哈希 返回block的地址
    func NewBlock(data string,prevBlockHash []byte) *Block{
    	block := &Block{
    		Timestamp:time.Now().Unix(),
    		PrevBlockHash:prevBlockHash,
    		Hash:[]byte{},
    		Data:[]byte(data),
    		Nonce:0}
    	pow := NewProofOfWork(block)
    	nonce,hash := pow.Run()
    	block.Hash = hash[:]
    	block.Nonce = nonce
    	return  block
    }

    func NewGenesisBlock() *Block {
    	return NewBlock("Genesis Block", []byte{})
    }

    func NewBlockChain() *BlockChain{
    	return &BlockChain{[]*Block{NewGenesisBlock()}}
    }

    func (blockchain *BlockChain) AddBlock(data string){
    	prevBlock := blockchain.blocks[len(blockchain.blocks)-1]
    	newBlock := NewBlock(data,prevBlock.Hash)
    	blockchain.blocks = append(blockchain.blocks,newBlock)
    }

    func main() {
    	blockchain := NewBlockChain()
    	blockchain.AddBlock("Send 1 BTC to Silver")
    	blockchain.AddBlock("Send 2 more BTC to Silver")

    	for _, block := range blockchain.blocks {
    		fmt.Printf("Prev hash: %x\n", block.PrevBlockHash)
    		fmt.Printf("Data: %s\n", block.Data)
    		fmt.Printf("Hash: %x\n", block.Hash)
    		pow := NewProofOfWork(block)
    		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
    		fmt.Println()
    	}
    }