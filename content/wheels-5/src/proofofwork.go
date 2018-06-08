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
