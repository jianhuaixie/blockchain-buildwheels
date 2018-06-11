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

// 构建一个POW的方法，很简单，巧妙的比较block与target
func NewProofOfWork(block *Block) *ProofOfWork{
	target := big.NewInt(1)
	target.Lsh(target,uint(256-targetBits))
	pow := &ProofOfWork{block,target}
	return pow
}

// 为工作量证明准备数据
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

// 工作量证明方法  返回nonce和hash
func (pow *ProofOfWork) Run() (int,[]byte){
	// 第一步，申明两个变量，一个用来跟target进行比较，一个用来表示hash结果
	var hashInt big.Int
	var hash	[32]byte
	nonce := 0
	fmt.Printf("Mining a new block")
	// 第二步，进行一个有限循环，不断尝试找最接近target值的nonce
	for nonce < maxNonce{
		data := pow.prepareData(nonce)
		hash = sha256.Sum256(data)
		if math.Remainder(float64(nonce),100000)==0{
			fmt.Printf("\r%x", hash)
		}
		hashInt.SetBytes(hash[:])
		if hashInt.Cmp(pow.target)==-1{
			break
		}else{
			nonce++
		}
	}
	fmt.Print("\n\n")
	return nonce,hash[:]
}

// 验证pow工作
func (pow *ProofOfWork) Validate() bool{
	var hashInt big.Int
	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])
	isValid := hashInt.Cmp(pow.target)==-1
	return isValid
}