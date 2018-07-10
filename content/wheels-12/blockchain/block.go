package blockchain

import (
	"bytes"
	"encoding/gob"
	"log"
)

type Block struct {
	Height int64
	Timestamp int64
	Amount int64
	Hash string
	PrevHash string
	GeneratedBy string
}

// serialize the block
func (block *Block) Serialize() []byte{
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(block)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}
// deserializeBlock a block
func DeserializeBlock(encodedBlock []byte) *Block{
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(encodedBlock))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}