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