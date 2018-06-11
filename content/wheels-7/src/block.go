package main

import (
	"time"
	"bytes"
	"encoding/gob"
	"log"
)

type Block struct {
	Timestamp	int64
	Transactions []*Transaction
	PrevBlockHash	[]byte
	Hash	[]byte
	Nonce	int
	Height	int
}

// 构建一个block需要三个元素：所有的交易记录，上一个块的hash，还有高度
func NewBlock(transactions []*Transaction,prevBlockHash []byte,height int) *Block{
	// 第一步就是构建一个block出来
	block := &Block{time.Now().Unix(),transactions,prevBlockHash,[]byte{},0,height}
	// 第二步就是进行工作量的证明,得到这个块的随机数nonce和哈希值
	pow := NewProofOfWork(block)
	nonce,hash := pow.Run()
	// 第三步，确定这个块的nonce和hash值
	block.Hash = hash[:]
	block.Nonce = nonce
	return block
}

// 构建一个初始化块，需要三个参数，只有coinbase的交易记录，上一块的hash为空，高位为0
func NewGenesisBlock(coinbase *Transaction) *Block{
	return NewBlock([]*Transaction{coinbase},[]byte{},0)
}

// 将一个块进行序列化转成字节数组
func (block *Block) Serialize() []byte{
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(block)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// 将一个序列化的块转成Block结构
func DeserializeBlock(d []byte) *Block{
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}

// 将一个块进行hash
func (block *Block) HashTransactions() []byte{
	// 第一步，声明一个能装所有交易的数组
	var transactions [][]byte
	// 第二步，遍历区块的所有交易，将交易装进数组
	for _,tx := range block.Transactions{
		transactions = append(transactions,tx.Serialize())
	}
	// 第三步，用所有的交易新建一个Merkle树
	merkleTree := NewMerkleTree(transactions)
	// 第四步，得到Merkle树的根节点的数据就能代表整个块的唯一hash值
	return merkleTree.RootNode.Data
}