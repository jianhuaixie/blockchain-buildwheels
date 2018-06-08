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