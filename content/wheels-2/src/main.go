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
const targetBits = 25
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