package main

import (
	"github.com/boltdb/bolt"
	"fmt"
	"os"
	"log"
	"bytes"
	"github.com/pkg/errors"
	"encoding/hex"
	"crypto/ecdsa"
)

const dbFile = "blockchain_%s.db"
const blocksBucket = "blocks"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

// Blockchain结构实际上有继承DB
type Blockchain struct {
	tip []byte
	db *bolt.DB
}

// 区块链迭代器 由当前hash值和数据库连接构成
type BlockchainIterator struct{
	currentHash []byte
	db	*bolt.DB
}

// 构建一条链，需要一个地址，和一个nodeID的参数
func CreateBlockchain(address,nodeID string) *Blockchain{
	// 第一步，通过nodeID来构建一个在本地保存的数据路径
	dbFile := fmt.Sprintf(dbFile, nodeID)
	if dbExists(dbFile){
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}
	// 第二步，构建一个初始块，在构建初始块之前，要先构建初始化块中的交易记录
	var tip []byte
	cbtx := NewCoinbaseTX(address,genesisCoinbaseData)
	genesis := NewGenesisBlock(cbtx)
	// 第三步，打开数据库，更新数据库，包括序列化后的块，还有块的hash
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
	// 第四步，将tip和db用来构建一个初始化的blockchain
	blockchain := Blockchain{tip,db}
	return &blockchain
}

// 创建一个区块链
func NewBlockchain(nodeID string) *Blockchain{
	// 检查是否已经有区块链数据库，如果没有，提醒创建
	dbFile := fmt.Sprintf(dbFile, nodeID)
	if dbExists(dbFile) == false {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}
	// 在区块链数据库中查出tip
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
	// 用tip构建一个区块链对象，然后返回
	blockchain := Blockchain{tip,db}
	return &blockchain
}

// 向区块链中添加块
func (blockchain *Blockchain) AddBlock(block *Block){
	err := blockchain.db.Update(func(tx *bolt.Tx) error {
		// 第一步，找到数据库，然后查找这个块是否已经存在
		bucket := tx.Bucket([]byte(blocksBucket))
		blockInDb := bucket.Get(block.Hash)
		if blockInDb != nil{
			return nil
		}
		// 第二步，将块序列化后存到数据库
		blockData := block.Serialize()
		err := bucket.Put(block.Hash,blockData)
		if err != nil {
			log.Panic(err)
		}
		// 第三步，拿出上一个块的hash，并取出上一个块
		lastHash := bucket.Get([]byte("1"))
		lastBlockData := bucket.Get(lastHash)
		lastBlock := DeserializeBlock(lastBlockData)
		// 第四步，比较两个块的高度，如果块高比上一个块更高，就更新最新块的hash为此块的hash值，并将区块链的tip也标记一下
		if block.Height>lastBlock.Height{
			err := bucket.Put([]byte("1"),block.Hash)
			if err != nil {
				log.Panic(err)
			}
			blockchain.tip = block.Hash
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// 根据交易ID找到交易
func (blockchain *Blockchain) FindTransaction(ID []byte)(Transaction,error){
	// 第一步，通过区块链迭代器将所有块放到迭代器里面
	blockchainiterator := blockchain.Iterator()
	// 迭代整个区块链迭代器，将块一个一个取出来
	for {
		block := blockchainiterator.Next()
		// 将块中的所有交易找出来，进行遍历，并将交易ID跟输入参数ID比较
		for _,tx := range block.Transactions{
			if bytes.Compare(tx.ID,ID) == 0{
				return *tx,nil
			}
		}
		if len(block.PrevBlockHash)==0{
			break
		}
	}
	// 第四步，如果没有找到，构建未找到的返回信息
	return Transaction{},errors.New("Transaction is not found")
}

// 找到所有未花费的交易输出
func (blockchain *Blockchain) FindUTXO() map[string]TXOutputs{
	// 第一步，申明两个对象，一个用来装TXOutputs的map 一个用来装已花费交易输出的索引数组的map
	UTXO := make(map[string]TXOutputs)		// key是交易输出ID，value是TXOutputs
	spentTXOs := make(map[string][]int)  	// key是交易输出ID，value是交易输出的索引数组
	// 第二步，从区块链得到区块链迭代器，并进行迭代,一个一个块取出
	blockchainiterator := blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		// 第三步，将每一个块中所有交易取出进行遍历
		for _,tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)  						// 交易ID
		// 第四步，检查每笔交易中的交易速出
		Outputs:
			for outIdx, out := range tx.Vout {  					// Vout由两部分组成，价值和公钥hash，此处outIdx为索引号，out为Vout
				if spentTXOs[txID] != nil{  						// 根据交易ID在已花费交易输出集合中找
					for _,spentOutIdx := range spentTXOs[txID]{	// 不为空，遍历这个索引数组
						if spentOutIdx == outIdx{					// 同一个交易中，找到此索引位置的交易输出已经是花费掉的
							continue Outputs						// 接着循环其他的交易输出
						}
					}
				}
				// 第五步，将未花费的输出装进UTXO集合map中
				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs,out)
				UTXO[txID] = outs
			}
			// 第六步,如果不是创世块，要将交易输入取出进行遍历
			if tx.IsCoinbase() == false{
				for _,in := range tx.Vin{
					// 第七步，将交易输入中的交易输出索引装进spentTXOs
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID],in.Vout)
				}
			}
		}
		// 第八步，到了初始块就停止遍历
		if len(block.PrevBlockHash)==0{
			break
		}
	}
	return UTXO
}

// 根据最新区块，返回其高度
func (blockchain *Blockchain) GetBestHeight() int{
	var lastBlock Block
	err := blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		lastHash := bucket.Get([]byte("1"))
		blockData := bucket.Get(lastHash)
		lastBlock = *DeserializeBlock(blockData)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return lastBlock.Height
}

// 根据block的hash，返回Block
func (blockchain *Blockchain) GetBlock(blockHash []byte) (Block,error){
	var block Block
	err := blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		blockData := bucket.Get(blockHash)
		if blockData == nil{
			return errors.New("Block is not found.")
		}
		block = *DeserializeBlock(blockData)
		return nil
	})
	if err != nil {
		return block, err
	}
	return block,nil
}

// 获取区块链的所有块的hash值
func (blockchain *Blockchain) GetBlockHashes() [][]byte{
	var blocks [][]byte
	blockchainiterator := blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		blocks = append(blocks,block.Hash)
		if len(block.PrevBlockHash)==0{
			break
		}
	}
	return blocks
}

// 根据提供的交易集合，然后进行挖矿，返回一个新的区块
func (blockchain *Blockchain) MineBlock(transactions []*Transaction) *Block{
	// 第一步，构建一个新的区块，需要两个参数，一个是最后一个区块的hash值，一个是块高度，所以，首先就申明这两个变量
	var lastHash []byte
	var lastHeight int
	// 第二步，检查输入的交易是否合法
	for _,tx := range transactions{
		// TODO: ignore transaction if its not valid
		if blockchain.VerifyTransaction(tx) != true{
			log.Panic("ERROR: Invalid transaction")
		}
	}
	// 第三步,从数据库中找到lastHash和lastHeight
	err :=  blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		lastHash = bucket.Get([]byte("1"))
		blockData := bucket.Get(lastHash)
		block := DeserializeBlock(blockData)
		lastHeight = block.Height
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	// 第四步，构建一个新的块
	newBlock := NewBlock(transactions,lastHash,lastHeight+1)
	// 第五步，更新到数据库,返回块
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
	return newBlock
}

// 用私钥对交易进行签名
func (blockchain *Blockchain) SignTransaction(tx *Transaction,privKey ecdsa.PrivateKey){
	prevTXs := make(map[string]Transaction)
	for _,vin := range tx.Vin{
		prevTX,err := blockchain.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	tx.Sign(privKey,prevTXs)
}

// 通过区块链迭代器，返回下一个区块
func (i *BlockchainIterator) Next() *Block{
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

// 返回一个区块链迭代器 通过区块链，找到当前区块链的tip，就能构建一个区块链迭代器
func (blockchain *Blockchain) Iterator() *BlockchainIterator{
	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
	return blockchainiterator
}
func (blockchain *Blockchain) VerifyTransaction(tx *Transaction) bool{
	// 第一步，判断是否交易是否是初始块中的交易，如果是，直接返回true
	if tx.IsCoinbase(){
		return true
	}
	// 第二步，声明一个map用来装Transaction
	prevTXs := make(map[string]Transaction)
	// 第三步，遍历交易的交易输入，通过交易ID找到交易，然后放入上面申明的map中
	for _,vin := range tx.Vin{
		prevTX,err := blockchain.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	// 第四步，通过交易的Verify方法，将交易输入集合map作为参数传入进行验证
	return tx.Verify(prevTXs)
}

func dbExists(dbFile string) bool{
	if _,err := os.Stat(dbFile);os.IsNotExist(err){
		return false
	}
	return true
}