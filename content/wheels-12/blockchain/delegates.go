package blockchain

import (
	"github.com/boltdb/bolt"
	"bytes"
	"encoding/gob"
	"log"
	"errors"
)

type Delegates struct {
	Version int64
	LastHeight int64
	Address string
	NumPeer int
}

const peerBucket = "peerBucket"

func GetDelegates(blockchain *Blockchain) []*Delegates{
	var listDelegate []*Delegates
	blockchain.db.View(func(tx *bolt.Tx) error {
		// assume bucket exists and has keys
		bucket := tx.Bucket([]byte(peerBucket))
		cursor := bucket.Cursor()
		for k,v := cursor.First();k!=nil;k,v=cursor.Next(){
			delegate := DeserializePeer(v)
			listDelegate = append(listDelegate,delegate)
		}
		return nil
	})
	return listDelegate
}

// serializes the delegates
func (delegates *Delegates) SerializeDelegate() []byte{
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(delegates)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}
// deserializes a delegates
func DeserializePeer(encodedDelegates []byte) *Delegates{
	var delegates Delegates
	decoder := gob.NewDecoder(bytes.NewReader(encodedDelegates))
	err := decoder.Decode(&delegates)
	if err != nil {
		log.Panic(err)
	}
	return &delegates
}

func UpdateDelegate(blockchain *Blockchain,address string,lastHeight int64){
	var delegate Delegates
	blockchain.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(peerBucket))
		delegateData := bucket.Get([]byte(address))
		if delegateData == nil{
			return errors.New("Delegates is not found")
		}
		delegate = *DeserializePeer(delegateData)
		if delegate.LastHeight < lastHeight{
			delegate.LastHeight = lastHeight
			bucket.Put([]byte(address),delegate.SerializeDelegate())
			log.Println("updated", address, lastHeight, delegate)
		}
		return nil
	})
}

// get numbber delegates
func GetNumberDelegates(blockchain *Blockchain) int{
	numberDelegate := 0
	blockchain.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(peerBucket))
		cursor := bucket.Cursor()
		for k,_ := cursor.First();k!=nil;k,_=cursor.Next(){
			numberDelegate += 1
		}
		return nil
	})
	return numberDelegate
}

func InsertDelegates(blockchain *Blockchain,delegate *Delegates,lastHeight int64) bool{
	isInsert := false
	err := blockchain.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(peerBucket))
		delegateData := bucket.Get([]byte(delegate.Address))
		if delegateData == nil{
			if delegate.LastHeight<lastHeight{
				delegate.LastHeight = lastHeight
			}
			err := bucket.Put([]byte(delegate.Address),delegate.SerializeDelegate())
			if err != nil {
				log.Panic(err)
			}
			isInsert = true
			return err
		}else{
			tmpDelegate := *DeserializePeer(delegateData)
			if tmpDelegate.LastHeight < lastHeight{
				delegate.LastHeight = lastHeight
				err := bucket.Put([]byte(delegate.Address),delegate.SerializeDelegate())
				if err != nil {
					log.Panic(err)
				}
				isInsert = true
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return isInsert
}