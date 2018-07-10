package database

import (
	"github.com/blockchain-buildwheels/content/wheels-9/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
)

type Cursor interface {
	Bucket()	Bucket
	Delete()	error
	First()	bool
	Last()	bool
	Next()	bool
	Prev()	bool
	Seek(seek []byte)	bool
	Key()	[]byte
	Value()	[]byte
}

type Bucket interface {
	Bucket(key []byte) Bucket
	CreateBucket(key []byte)(Bucket,error)
	CreateBucketIfNotExists(key []byte)(Bucket,error)
	DeleteBucket(key []byte) error
	ForEach(func(k,v []byte) error) error
	ForEachBucket(func(k []byte) error) error
	Cursor() Cursor
	Writable()	bool
	Put(key,value []byte) error
	Get(key []byte) []byte
	Delete(key []byte) error
}

type BlockRegion struct {
	Hash *chainhash.Hash
	Offset	uint32
	Len	uint32
}

type Tx interface {
	Metadata()	Bucket
	StoreBlock(block *btcutil.Block) error
	HasBlock(hash *chainhash.Hash) (bool,error)
	HasBlocks(hashes []chainhash.Hash) ([]bool,error)
	FetchBlockHeader(hash *chainhash.Hash) ([]byte,error)
	FetchBlockHeaders(hashes []chainhash.Hash) ([][]byte,error)
	FetchBlock(hash *chainhash.Hash) ([]byte,error)
	FetchBlocks(hashes []chainhash.Hash) ([][]byte,error)
	FetchBlockRegion(region *BlockRegion) ([]byte,error)
	FetchBlockRegions(regions []BlockRegion) ([][]byte,error)
	Commit() error
	Rollback() error
}

type DB interface {
	Type() string
	Begin(writable bool) (Tx,error)
	View(fn func(tx Tx) error) error
	Update(fn func(tx Tx) error) error
	Close() error
}