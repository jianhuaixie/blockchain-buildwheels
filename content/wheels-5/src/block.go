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