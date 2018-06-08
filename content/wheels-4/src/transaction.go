package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"crypto/sha256"
	"fmt"
	"encoding/hex"
)

const subsidy = 10 //是挖出新块的奖励金

type Transaction struct {
	ID	[]byte
	Vin	[]TXInput
	Vout	[]TXOutput
}

type TXInput struct {
	Txid	[]byte  	// 一个交易输入引用之前一笔交易的一个输出
	Vout	int			//一笔交易可能有多个输出，Vout为输出的索引
	ScriptSig	string	//提供解锁输出Txid:Vout的数据
}

type TXOutput struct {
	Value	int				//有多少币
	ScriptPubKey	string	//对输出进行锁定
}

func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}
func (tx *Transaction) SetID(){
	var encoded bytes.Buffer
	var hash [32]byte
	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}
func (in *TXInput) CanUnlockOutputWith(unlockingData string) bool{
	return in.ScriptSig == unlockingData
}
func (out *TXOutput) CanBeUnlockedWith(unlockingData string) bool{
	return out.ScriptPubKey == unlockingData
}
// NewCoinbaseTX 构建coinbase交易，没有输入，只有一个输出
func NewCoinbaseTX(to,data string) *Transaction{
	if data == "" {
		data = fmt.Sprintf("Reward to '%s'", to)
	}
	txin := TXInput{[]byte{},-1,data}
	txout := TXOutput{subsidy,to}
	tx := Transaction{nil,[]TXInput{txin},[]TXOutput{txout}}
	tx.SetID()
	return &tx
}
// NewUTXOTransaction 创建一笔新的交易
func NewUTXOTransaction(from,to string,amount int,blockchain *Blockchain) *Transaction{
	var inputs []TXInput
	var outputs []TXOutput
	// 找到足够的未花费输出
	acc,validOutputs := blockchain.FindSpendableOutputs(from,amount)
	if acc<amount{
		log.Panic("ERROR:Not enough funds")
	}
	for txid,outs := range validOutputs {
		txID,err := hex.DecodeString(txid)
		if err!=nil{
			log.Panic(err)
		}
		for _,out := range outs{
			input := TXInput{txID,out,from}
			inputs = append(inputs,input)
		}
	}
	outputs = append(outputs,TXOutput{amount,to})
	// 如果UTXO总数超过所需，则产生找零
	if acc>amount{
		outputs = append(outputs,TXOutput{acc-amount,from})
	}
	tx := Transaction{nil,inputs,outputs}
	tx.SetID()
	return &tx
}