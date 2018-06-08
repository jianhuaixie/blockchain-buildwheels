### 区块

首先来一个简化版本的区块，仅仅包含最关键的信息，其数据结构在go语言定义如下：

    type Block struct {
        Timestamp   int64
        Data    []byte
        PrevBlockHash   []byte
        Hash    []byte
    }

我们先不管复杂的区块头是如何定义的，Hash字段是如何计算的呢？

    Hash = SHA256(PrevBlockHash + Timestamp + Data)

### 区块链

区块链就是将区块有序链起来，然后通过区块的hash能快速检索到一个块。

为了实现链起区块和检索区块的功能，在go语言里，用一个array和map的数据结构就能解决问题。

    type Blockchain struct {
        blocks []*Block
    }

### 简单的轮子版本一

区块链原型：一个数组存了一些有序的区块，每一个区块都跟前一个区块相关联，这里加入新的区块是很容易的，但真实的区块链要复杂多，
需要工作量证明（PoW），当然本质上是一个共识机制，就是一个节点添加区块，需要获得其他节点的认同，从而形成的一个共识（consensus）。

main.go

        package main

        import (
        	"time"
        	"strconv"
        	"bytes"
        	"crypto/sha256"
        	"fmt"
        )

        type Block struct {
        	Timestamp	int64
        	PrevBlockHash	[]byte
        	Hash	[]byte
        	Data	[]byte
        }

        type BlockChain struct {
        	blocks []*Block
        }

        func NewBlock(data string,prevBlockHash []byte) *Block{
        	block := &Block{
        		Timestamp:time.Now().Unix(),
        		PrevBlockHash:prevBlockHash,
        		Hash:[]byte{},
        		Data:[]byte(data)}
        	block.SetHash()
        	return block
        }

        // Hash = sha256(PrevBlockHash + Data + Timestamp)
        func (b *Block) SetHash(){
        	timestamp := []byte(strconv.FormatInt(b.Timestamp,10))
        	headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data, timestamp}, []byte{})
        	hash := sha256.Sum256(headers)
        	b.Hash = hash[:]
        }

        func NewGenesisBlock() *Block{
        	return NewBlock("Genesis Block",[]byte{})
        }

        // 创建一个有创世块的链
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
        	blockchain.AddBlock("send 2 BTC to Silver again")
        	for _,block := range blockchain.blocks{
        		fmt.Printf("Prev hash: %x\n",block.PrevBlockHash)
        		fmt.Printf("Data: %s\n", block.Data)
        		fmt.Printf("Hash: %x\n", block.Hash)
        		fmt.Println()
        	}
        }
















