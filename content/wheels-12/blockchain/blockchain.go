package blockchain

import (
	"github.com/boltdb/bolt"
	"log"
	"github.com/pkg/errors"
	"fmt"
	"crypto/sha256"
	"encoding/hex"
	"time"
	"github.com/blockchain-buildwheels/content/wheels-12/utils"
)

const dbFile = "blockchain_%s.db"
const blocksBucket = "blocks"

// blockchain implements interactons with a DB
type Blockchain struct {
	tip []byte
	db *bolt.DB
}

// BlockchainIterator is used to iterate over blockchain blocks
type BlockchainIterator struct {
	currentHash []byte
	db 	*bolt.DB
}

// AddBlock saves the block into the blockchain
func (blockchain *Blockchain) AddBlock(block *Block){
	err := blockchain.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		hash := []byte(block.Hash)
		blockInDb := bucket.Get(hash)
		if blockInDb != nil{
			return nil
		}
		blockData := block.Serialize()
		err := bucket.Put(hash,blockData)
		if err != nil {
			log.Panic(err)
		}
		lastHash := bucket.Get([]byte("lastHash"))
		lastBlockData := bucket.Get(lastHash)
		lastBlock := DeserializeBlock(lastBlockData)
		if block.Height>lastBlock.Height{
			err = bucket.Put([]byte("lastHash"),hash)
			if err != nil {
				log.Panic(err)
			}
			blockchain.tip = hash
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// GetBestHeight returns the height of the latest block
func (blockchain *Blockchain) GetBestHeight() int64{
	var lastBlock Block
	err := blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		lastHash := bucket.Get([]byte("lastHash"))
		blockData := bucket.Get(lastHash)
		lastBlock = *DeserializeBlock(blockData)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return lastBlock.Height
}

// GetLastHeight returns the height of the latest block
func (blockchain *Blockchain) GetLastBlock() Block{
	var lastBlock Block
	err := blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		lastHash := bucket.Get([]byte("lastHash"))
		blockData := bucket.Get(lastHash)
		lastBlock = *DeserializeBlock(blockData)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return lastBlock
}

// Iterator returns a BlockchainIterator
func (blockchain *Blockchain) Iterator() *BlockchainIterator{
	blockchainiterator := &BlockchainIterator{blockchain.tip,blockchain.db}
	return blockchainiterator
}

// GetBlock finds a block by its hash and returns it
func (blockchain *Blockchain) GetBlock(blockHash []byte) (Block,error){
	var block Block
	err := blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		blockData := bucket.Get(blockHash)
		if blockData == nil{
			return errors.New("Block is not found")
		}
		block = *DeserializeBlock(blockData)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return block,nil
}

// Next returns next block starting from the tip
func(blockchainiterator *BlockchainIterator) Next() *Block{
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
	blockchainiterator.currentHash = []byte(block.PrevHash)
	return block
}

// GetBlockhashes returns a list of hashes of all the blocks in the chain
func (blockchain *Blockchain) GetBlockHashes() [][]byte{
	var blocks [][]byte
	blockchainiterator := blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		blocks = append(blocks,[]byte(block.Hash))
		if len(block.PrevHash)==0{
			break
		}
	}
	return blocks
}

func setupDB(dbName string) (*bolt.DB,error){
	db,err := bolt.Open(dbName,0600,nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_,err := tx.CreateBucketIfNotExists([]byte(blocksBucket))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		return nil
	})
	err = db.Update(func(tx *bolt.Tx) error {
		_,err := tx.CreateBucketIfNotExists([]byte(peerBucket))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		return nil
	})
	return db,nil
}

// calculateHash is a simple SHA256 hashing function
func calculateHash(s string) string{
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// calculateBlockHash returns the hash of all block information
func calculateBlockHash(block *Block) string{
	record := string(block.Height)+ string(block.Timestamp) +  string(block.Amount) + block.PrevHash
	return calculateHash(record)
}

// generateBlock creates a new block using previous block's hash
func generateBlock(oldBlock Block,Amout int64,GeneratedBy string) *Block{
	newBlock := &Block{}
	t := time.Now()
	newBlock.Height = oldBlock.Height+1
	newBlock.Amount = Amout
	newBlock.Timestamp = t.Unix()
	newBlock.GeneratedBy = GeneratedBy
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateBlockHash(newBlock)
	return newBlock
}

// generate genesis block
func genBlock(db *bolt.DB) *Block{
	block := &Block{0,time.Now().Unix(),0,"","","root"}
	block.Hash = calculateBlockHash(block)
	return block
}

// NewBlockchain creates a new Blockchain with genesis Block
func NewBlockchain(nodeID string) *Blockchain{
	dbFile := fmt.Sprintf(dbFile,nodeID)
	db,err := setupDB(dbFile)
	var tip []byte
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blocksBucket))
		tip = bucket.Get([]byte("lastHash"))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	if len(tip) == 0{
		block := genBlock(db)
		err = db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(blocksBucket))
			err = bucket.Put([]byte("lastHash"),[]byte(block.Hash))
			err = bucket.Put([]byte(block.Hash),block.Serialize())
			if err != nil {
				log.Panic(err)
			}
			return nil
		})
		tip = []byte(block.Hash)
	}
	fmt.Println(tip)
	blockchain := Blockchain{tip,db}
	return &blockchain
}

func genBlockPeriod(blockchain *Blockchain){
	time.Sleep(utils.BLOCK_TIME*time.Second)
	listDelegates := GetDelegates(blockchain)
	if len(listDelegates) > 3 {
		lastBlock := blockchain.GetLastBlock()
		indexDelegate := 0
		delegate := listDelegates[indexDelegate]
		if nodeAddress == knownNodes[0]{
			quorum := 0
			noquorum := 0
			for _,delegatePeer := range listDelegates{
				if delegatePeer.LastHeight == lastBlock.Height{
					quorum += 1
				}else{
					noquorum += 1
				}
			}
			log.Println(quorum,noquorum)
			if (quorum+noquorum)>0 && float64(float64(quorum)/float64(quorum+noquorum))>float64(0.66){
				block := generateBlock(lastBlock,0,delegate.Address)
				blockchain.AddBlock(block)
				log.Println(block)

				for _,delegatePeer := range listDelegates{
					if delegatePeer.Address != nodeAddress{
						log.Println("sendBlock",delegatePeer.Address)
						SendBlock(delegatePeer.Address,block)
						UpdateDelegate(blockchain,delegatePeer.Address,block.Height)
					}
				}
			}
		}else{
			log.Println(delegate.Address)
		}
	}else {
		log.Println("len peer: ",len(listDelegates))
	}
}

// Delegates Proof of State
func Forks(blockchain *Blockchain){
	for {
		genBlockPeriod(blockchain)
	}
}
