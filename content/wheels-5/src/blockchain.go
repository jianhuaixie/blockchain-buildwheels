package main

import (
	"github.com/boltdb/bolt"
	"fmt"
	"os"
	"log"
	"encoding/hex"
	"bytes"
	"github.com/pkg/errors"
	"crypto/ecdsa"
)

const dbFile = "blockchain.db"
const blocksBucket = "blocks"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

type Blockchain struct {
	tip	[]byte
	db	*bolt.DB
}
type BlockchainIterator struct {
	currentHash	[]byte
	db	*bolt.DB
}
func CreateBlockchain(address string) *Blockchain{
	if dbExists(){
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}
	var tip []byte
	cbtx := NewCoinbaseTX(address,genesisCoinbaseData)
	genesis := NewGenesisBlock(cbtx)
	db,err := bolt.Open(dbFile,0600,nil)
	if err != nil {
		log.Panic(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
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
	if err != nil {
		log.Panic(err)
	}
	blockchain := Blockchain{tip,db}
	return &blockchain
}
// finds and returns unspent outputs to reference in inputs
func (blockchain *Blockchain) FindSpendableOutputs(pubKeyHash []byte,amount int) (int,map[string][]int){
	unspentOutputs := make(map[string][]int)
	unspentTXs := blockchain.FindUnspentTransactions(pubKeyHash)
	accumulated := 0
Work:
	for _,tx := range unspentTXs{
		txID := hex.EncodeToString(tx.ID)
		for outIdx,out := range tx.Vout{
			if out.IsLockedWithKey(pubKeyHash) && accumulated<amount{
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
// finds a transaction by its ID
func (blockchain *Blockchain) FindTransaction(ID []byte) (Transaction,error){
	blockchainiterator := blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		for _,tx := range block.Transactions{
			if bytes.Compare(tx.ID,ID)==0{
				return *tx,nil
			}
		}
		if len(block.PrevBlockHash)==0{
			break
		}
	}
	return Transaction{},errors.New("Transaction is not found")
}
// returns da list of transactions containing unspent outputs
func (blockchain *Blockchain) FindUnspentTransactions(pubKeyHash []byte) []Transaction{
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)
	blockchainiterator := blockchain.Iterator()
	for{
		block := blockchainiterator.Next()
		for _,tx := range block.Transactions{
			txID := hex.EncodeToString(tx.ID)
		Outputs:
			for outIdx,out := range tx.Vout{
				if spentTXOs[txID] != nil{
					for _,spentOutIdx := range spentTXOs[txID]{
						if spentOutIdx==outIdx{
							continue Outputs
						}
					}
				}
				if out.IsLockedWithKey(pubKeyHash){
					unspentTXs = append(unspentTXs,*tx)
				}
			}
			if tx.IsCoinbase() == false{
				for _,in := range tx.Vin{
					if in.UsesKey(pubKeyHash){
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
// finds and returns all unspent transaction outputs
func (blockchain *Blockchain) FindUTXO(pubKeyHash []byte) []TXOutput{
	var UTXOs []TXOutput
	unspentTransactions := blockchain.FindUnspentTransactions(pubKeyHash)
	for _,tx := range unspentTransactions{
		for _,out := range tx.Vout{
			if out.IsLockedWithKey(pubKeyHash){
				UTXOs = append(UTXOs,out)
			}
		}
	}
	return UTXOs
}
func (blockchain *Blockchain) Iterator() *BlockchainIterator{
	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
	return blockchainiterator
}
// mines a new block with the provided transactions
func (blockchain *Blockchain) MineBlock(transactions []*Transaction){
	var lastHash []byte
	for _,tx := range transactions{
		if blockchain.VerifyTransaction(tx) != true{
			log.Panic("ERROR: Invalid transaction")
		}
	}
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
	if err != nil {
		log.Panic(err)
	}
}

// signs inputs of a Transaction
func (blockchain *Blockchain) SignTransaction(tx *Transaction,privKey ecdsa.PrivateKey){
	prevTXs := make(map[string]Transaction)
	for _,vin := range tx.Vin{
		prevTX,err := blockchain.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)]=prevTX
	}
	tx.Sign(privKey,prevTXs)
}

// verifies transaction input signatures
func (blockchain *Blockchain) VerifyTransaction(tx *Transaction) bool{
	prevTXs := make(map[string]Transaction)
	for _,vin := range tx.Vin{
		prevTX,err := blockchain.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	return tx.Verify(prevTXs)
}

func (blockchainiterator *BlockchainIterator) Next() *Block{
	var block *Block
	err := blockchainiterator.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		encodedBlock := bucket.Get(blockchainiterator.currentHash)
		block = DeserializeBlock(encodedBlock)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	blockchainiterator.currentHash = block.PrevBlockHash
	return block
}
func dbExists() bool{
	if _,err := os.Stat(dbFile);os.IsNotExist(err){
		return false
	}
	return true
}
