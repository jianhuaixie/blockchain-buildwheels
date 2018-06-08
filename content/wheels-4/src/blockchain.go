package main

import (
	"github.com/boltdb/bolt"
	"log"
	"os"
	"fmt"
	"encoding/hex"
)

const dbFile = "blockchain.db"
const blocksBucket = "blocks"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

type Blockchain struct {
	tip []byte
	db *bolt.DB
}
type BlockchainIterator struct {
	currentHash	[]byte
	db	*bolt.DB
}
func (blockchain *Blockchain) MineBlock(transactions []*Transaction){
	var lastHash []byte
	err := blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		lastHash = bucket.Get([]byte("1"))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	newBlock := NewBlock(transactions,lastHash)
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
}
func (blockchain *Blockchain) Iterator() *BlockchainIterator{
	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
	return blockchainiterator
}
func (i *BlockchainIterator) Next() *Block {
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
func dbExists() bool{
	if _,err := os.Stat(dbFile);os.IsNotExist(err){
		return  false
	}
	return true
}
// 创建一个有创世块的新链
func NewBlockchain(address string) *Blockchain{
	if dbExists()==false{
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}
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
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		tip = bucket.Get([]byte("1"))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	blockchain := Blockchain{tip,db}
	return &blockchain
}
// 创建一个新的区块链数据库，address用来接收挖出创世块的奖励
func CreateBlockchain(address string) *Blockchain{
	if dbExists(){
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}
	var tip []byte
	db,err := bolt.Open(dbFile,0600,nil)
	if err != nil{
		log.Panic(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		cbtx := NewCoinbaseTX(address,genesisCoinbaseData)
		genesis := NewGenesisBlock(cbtx)
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
	blockchain := Blockchain{tip,db}
	return &blockchain
}
// 找到未花费输出的交易
func (blockchain *Blockchain) FindUnspentTransactions(address string) []Transaction{
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)
	blockchainiterator := blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		for _,tx := range block.Transactions{
			txID := hex.EncodeToString(tx.ID)
		Outputs:
			for outIdx,out := range tx.Vout{
				if spentTXOs[txID] != nil { // 如果交易输出被花费了
					for _,spentOut := range spentTXOs[txID]{
						if spentOut == outIdx{
							continue Outputs
						}
					}
				}
				// 如果该交易输出可以被解锁，即可倍花费
				if out.CanBeUnlockedWith(address){
					unspentTXs = append(unspentTXs,*tx)
				}
			}
			if tx.IsCoinbase() == false{
				for _,in := range tx.Vin{
					if in.CanUnlockOutputWith(address){
						inTxID := hex.EncodeToString(in.Txid)
						spentTXOs[inTxID] = append(spentTXOs[inTxID],in.Vout)
					}
				}
			}
		}
		if len(block.PrevBlockHash)==0{
			break
		}
	}
	return unspentTXs
}
func (blockchain *Blockchain) FindUTXO(address string) []TXOutput {
	var UTXOs []TXOutput
	unspentTransactions := blockchain.FindUnspentTransactions(address)
	for _,tx := range unspentTransactions{
		for _,out := range tx.Vout{
			if out.CanBeUnlockedWith(address){
				UTXOs = append(UTXOs,out)
			}
		}
	}
	return UTXOs
}
// 从address中找到至少amount的UTXO
func (blockchain *Blockchain) FindSpendableOutputs(address string,amount int) (int,map[string][]int){
	unspentOutputs := make(map[string][]int)
	unspentTXs := blockchain.FindUnspentTransactions(address)
	accumulated := 0
Work:
	for _,tx := range unspentTXs{
		txID := hex.EncodeToString(tx.ID)
		for outIdx,out := range tx.Vout{
			if out.CanBeUnlockedWith(address) && accumulated<amount{
				accumulated += out.Value
				unspentOutputs[txID] = append(unspentOutputs[txID],outIdx)
				if accumulated>=amount{
					break Work
				}
			}
		}
	}
	return accumulated,unspentOutputs
}