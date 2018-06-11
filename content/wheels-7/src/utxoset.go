package main

import (
	"github.com/boltdb/bolt"
	"log"
	"encoding/hex"
)

const utxoBucket = "chainstate"

// 未花费交易输出集合 ，实际上就是区块链Blockchain的一个子集合
type UTXOSet struct {
	Blockchain *Blockchain
}

// 通过pubKeyHash，从未花费交易输出集合中，找到属于这个pubKeyHash的未花费交易输出子集合
func (u UTXOSet) FindUTXO(pubKeyHash []byte) []TXOutput {
	// 第一步，申明一个交易输出数组用来装找到的未花费交易输出
	var UTXOs []TXOutput
	// 第二步，从未花费交易输出集合中获取数据库，然后从数据库中找到连接
	db := u.Blockchain.db
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoBucket))
		cursor := bucket.Cursor()
		// 第三步，遍历数据库中未花费交易输出，并将取出的未花费交易输出进行反序列化
		for k,v := cursor.First();k!=nil;k,v = cursor.Next(){
			outs := DeserializeOutputs(v)
			// 第四步，将取出的未花费交易输出进行遍历，然后跟pubKeyHash进行匹配验证，看这个交易输出是否属于这个pubKeyHash
			for _,out := range outs.Outputs{
				if out.IsLockedWithKey(pubKeyHash){
					UTXOs = append(UTXOs,out)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return UTXOs
}

// 对UTXO集合进行重新索引
func (u UTXOSet) Reindex() {
	// 第一步，从UTXO集和中找到数据库句柄
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)
	// 第二步，删除掉utxoBucket这个表，然后重新建立一个表
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound{
			log.Panic(err)
		}
		_,err = tx.CreateBucket(bucketName)
		if err!=nil{
			log.Panic(err)
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	// 第三步，从UTXO集合中找到Blockchain，用FindUTXO方法，找到新的UTXO集合
	UTXO := u.Blockchain.FindUTXO()
	// 第四步，对新UTXO集合进行遍历，再写入数据库
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		for txID,outs := range UTXO{
			key,err := hex.DecodeString(txID)
			if err != nil {
				log.Panic(err)
			}
			err = bucket.Put(key,outs.SerializeOutputs())
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
}

// 返回UTXO集合中交易的数量
func (set UTXOSet) CountTransactions() int {
	db := set.Blockchain.db
	counter := 0
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoBucket))
		cursor := bucket.Cursor()
		for k,_ := cursor.First();k!=nil;k,_=cursor.Next(){
			counter ++
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return counter
}

// 从UTXOSet中找到可以支付的，然后返回金额和交易输出集合
func (set *UTXOSet) FindSpendableOutput(pubKeyHash []byte, amount int) (int,map[string][]int) {
	// 第一步，声明一个用来装未花费交易输出的集合变量和金额变量
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	// 第二步，从UTXOSet中，拿到数据库的句柄，取出utxo集合进行遍历
	db := set.Blockchain.db
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoBucket))
		cursor := bucket.Cursor()
		for k,v := cursor.First();k!=nil;k,v = cursor.Next(){
			// 第三步，从未花费交易输出记录中拿到txID和outs
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)
			// 第四步，对outs进行遍历，如果属于pubKeyHash，且金额还未超额，就累加金额，且把未花费交易输出装到之前定义的集合变量
			for outIdx,out := range outs.Outputs{
				if out.IsLockedWithKey(pubKeyHash) && accumulated<amount{
					accumulated += out.Value
					unspentOutputs[txID] = append(unspentOutputs[txID],outIdx)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return accumulated,unspentOutputs
}

// 一个新的块，然后需要更新UTXO集合
func (set UTXOSet) Update(block *Block) {
	// 第一步，从UTXO集合拿到数据库句柄
	db := set.Blockchain.db
	// 第二步，更新数据库中的UTXO集合，从新块中取到交易记录集合进行遍历
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoBucket))
		for _,tx := range block.Transactions{
			// 第三步，判断是否是初始块中的交易
			if tx.IsCoinbase() == false{
				for _,vin := range tx.Vin{
					// 第四步，声明需要更新的交易输出变量
					updatedOuts := TXOutputs{}
					outsBytes := bucket.Get(vin.Txid)  		// 新块中的交易输入ID，可以从数据库中找到交易输出数据
					outs := DeserializeOutputs(outsBytes)	// 将交易输出数据进行反序列化，得到交易输出
					for outIdx,out := range outs.Outputs{	// 将交易输出进行遍历
						if outIdx != vin.Vout{				// 如果outIdx跟新块中的交易输入的交易输出索引不想等
							updatedOuts.Outputs = append(updatedOuts.Outputs,out)	// 就是需要更新的交易输出，装进之前声明的变量
						}
					}
					// 第五步，判断需要更新的交易输出集合如果为空，需要在数据库中将新块的交易输出删除掉
					if len(updatedOuts.Outputs)==0{
						err := bucket.Delete(vin.Txid)
						if err != nil {
							log.Panic(err)
						}
					}else{
						// 第六步，如果不为空，需要将需要更新的交易输出写到数据库
						err := bucket.Put(vin.Txid,updatedOuts.SerializeOutputs())
						if err != nil {
							log.Panic(err)
						}
					}
				}
			}
			// 第七步，声明一个新的交易输出变量
			newOutputs := TXOutputs{}
			// 第八步，遍历新块的交易输出，并且装进上一步声明的变量中
			for _,out := range tx.Vout{
				newOutputs.Outputs = append(newOutputs.Outputs,out)
			}
			// 第九步，将新块的交易输出写到数据库
			err := bucket.Put(tx.ID,newOutputs.SerializeOutputs())
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}
