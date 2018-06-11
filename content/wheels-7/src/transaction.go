package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"crypto/rand"
	"fmt"
	"crypto/sha256"
	"crypto/ecdsa"
	"encoding/hex"
	"strings"
	"crypto/elliptic"
	"math/big"
)

const subsidy = 10

// 每一笔交易都有一个ID，然后有一个交易输出数组，一个交易输出数组，其中交易输入中记录了交易输出中的输出索引，也就是说
// 交易输入中的输出索引是能找到币的去处的。而交易输出又被公钥锁定，每个交易输出都能知道属于谁。
type Transaction struct{
	ID	[]byte
	Vin	[]TXInput
	Vout	[]TXOutput
}

// TXInput的结构包含四个部分：id，Vout的索引，签名，公钥
type TXInput struct {
	Txid	[]byte
	Vout	int
	Signature	[]byte
	PubKey	[]byte
}

// TXOutput的结构包含两部分：价值，公钥hash
type TXOutput struct {
	Value	int
	PubKeyHash	[]byte
}

type TXOutputs struct {
	Outputs []TXOutput
}

// 用公钥地址hash值来检查交易输入
func (in *TXInput) UsesKey(pubKeyHash []byte) bool{
	lockingHash := HashPubKey(in.PubKey)
	return bytes.Compare(lockingHash,pubKeyHash)==0
}

// 将交易输出集合进行序列化
func (outs TXOutputs) SerializeOutputs() []byte{
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(outs)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// 将交易输出集合进行反序列化，转成交易输出集合TXOutputs
func DeserializeOutputs(data []byte) TXOutputs{
	var outputs TXOutputs
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&outputs)
	if err != nil {
		log.Panic(err)
	}
	return outputs
}

// 将交易序列化成字节数组
func (tx Transaction) Serialize() []byte{
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	return encoded.Bytes()
}

// 构建一个初始化块中的交易记录
func NewCoinbaseTX(to,data string) *Transaction{
	// 第一步，检查data是否为空，如果为空，构建一点随机数据进去
	if data == ""{
		randData := make([]byte,20)
		_,err := rand.Read(randData)
		if err != nil {
			log.Panic(err)
		}
		data = fmt.Sprintf("%x",randData)
	}
	// 第二步 构建TXInout，其Txid为空，Vout为-1，签名为nil，公钥为data
	txin := TXInput{[]byte{},-1,nil,[]byte(data)}
	txout := NewTXOutput(subsidy,to)
	tx := Transaction{nil,[]TXInput{txin},[]TXOutput{*txout}}
	tx.ID = tx.Hash()
	return &tx
}

// 构建TXOutput，参数就是value和address,其中公钥address就是锁定TXOutput，证明这笔花费是此地址的
func NewTXOutput(value int,address string) *TXOutput{
	txo := &TXOutput{value,nil}
	txo.Lock([]byte(address))
	return txo
}

// 对output进行签名锁定
func (out *TXOutput) Lock(address []byte){
	pubKeyHash := Base58Decode(address)
	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-4]
	out.PubKeyHash = pubKeyHash
}

// 验证交易输出是否属于这个pubKeyHash的
func (output TXOutput) IsLockedWithKey(pubKeyHash []byte) bool{
	return bytes.Compare(output.PubKeyHash,pubKeyHash)==0
}

// 将交易进行hash
func (tx *Transaction) Hash() []byte{
	var hash [32]byte
	txCopy := *tx
	hash = sha256.Sum256(txCopy.Serialize())
	return hash[:]
}

// 对一个交易集合中的交易输入用私钥进行签名
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey,prevTXs map[string]Transaction){
	// 第一步，判断是否为初始区块交易，如果是，直接返回
	if tx.IsCoinbase(){
		return
	}
	// 第二步，对交易中的交易输入进行遍历 查看是否所有交易都在实现准备好的交易输出集合中
	for _,vin := range tx.Vin{
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil{
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}
	// 第三步，修剪交易复制，然后对其中的交易输入进行遍历
	txCopy := tx.TrimmedCopy()
	for inID,vin := range txCopy.Vin {
		// 第四步，txCopy的Signatue为空，有公钥hash，然后用私钥进行签名
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]            // 实现准备好的交易
		txCopy.Vin[inID].Signature = nil                           // txCopy中的这个交易输入Signature为nil
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash // txCopy中的这个交易输入PubKey为实现准备好交易输入的PubKeyHash
		dataTooSign := fmt.Sprintf("%x\n", txCopy)
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(dataTooSign))
		if err != nil {
			log.Panic(err)
		}
		signature := append(r.Bytes(), s.Bytes()...)
		// 第五步，将签名赋值给tx的这个交易输入的Signature，然后将txCopy的PubKey赋值为nil
		tx.Vin[inID].Signature = signature
		txCopy.Vin[inID].PubKey = nil
	}
}

// 将一个交易转成人类可读的字符串形式
func (tx Transaction) String() string{
	var lines []string
	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	for i, input := range tx.Vin {
		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.Txid))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Vout))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
	}
	for i, output := range tx.Vout {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
	}
	return strings.Join(lines, "\n")
}

// 返回一个修剪过的交易复制，所谓修剪就是交易中交易输入Signature为nil,PubKey为nil
func (tx *Transaction) TrimmedCopy() Transaction{
	var inputs []TXInput
	var outputs []TXOutput
	for _,vin := range tx.Vin{
		inputs = append(inputs,TXInput{vin.Txid,vin.Vout,nil,nil})
	}
	for _,vout := range tx.Vout{
		outputs = append(outputs, TXOutput{vout.Value, vout.PubKeyHash})
	}
	txCopy := Transaction{tx.ID,inputs,outputs}
	return txCopy
}

// 判断交易是否为创世区块中的交易
func (tx Transaction) IsCoinbase() bool{
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

// 将交易输入集合进行验证
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	// 第一步，检查交易是否在初始块中，如果是，直接返回true
	if tx.IsCoinbase(){
		return true
	}
	// 第二步，检查需要验证的交易输入集合是否都是存在
	for _,vin := range tx.Vin{
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil{
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}
	// 第三步，将交易剪枝复制，声明一个椭圆加密对象
	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()
	// 第四步，将交易输入进行遍历,然后进行验证
	for inID,vin := range tx.Vin{
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		r := big.Int{}
		s := big.Int{}
		sigLen := len(vin.Signature)
		r.SetBytes(vin.Signature[:(sigLen / 2)])
		s.SetBytes(vin.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(vin.PubKey)
		x.SetBytes(vin.PubKey[:(keyLen / 2)])
		y.SetBytes(vin.PubKey[(keyLen / 2):])

		dataToVerify := fmt.Sprintf("%x\n", txCopy)

		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		if ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) == false {
			return false
		}
		txCopy.Vin[inID].PubKey = nil
	}
	return true
}

// 用钱包，目标地址，金额，未花费集合来构建一个未花费交易
func NewUTXOTransaction(wallet *Wallet, to string, amount int, UTXOSet *UTXOSet) *Transaction {
	// 第一步，声明交易输入数组和交易输出数组
	var inputs []TXInput
	var outputs []TXOutput
	// 第二步，从钱包中取出公钥hash
	pubKeyHash := HashPubKey(wallet.PublicKey)
	// 第三步，用公钥hash和所需支付amount去UTXOSet找到可以支付的，输出acc和可行的交易输出集合
	acc ,validOutputs := UTXOSet.FindSpendableOutput(pubKeyHash,amount)
	// 第四步，对可行的未花费交易输出进行遍历
	for txid,outs := range validOutputs{
		txID,err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}
		for _,out := range outs{
			input := TXInput{txID,out,nil,wallet.PublicKey}
			inputs = append(inputs,input)
		}
	}
	// 第五步，从钱包中拿到from地址
	from := fmt.Sprintf("%s", wallet.GetAddress())
	// 第六步，将要支付的加到交易输出集合
	outputs = append(outputs,*NewTXOutput(amount,to))
	// 第七步，如果还有找零，就将交易输出也添加到交易输出集合
	if acc<amount{
		outputs = append(outputs,*NewTXOutput(acc-amount,from))
	}
	// 第八步，有了交易输出集合和交易输入集合，就构建交易，将交易hash后作为交易的ID
	tx := Transaction{nil,inputs,outputs}
	tx.ID = tx.Hash()
	// 第九步，将UTXOSet的交易，用私钥进行签名，并且返回交易
	UTXOSet.Blockchain.SignTransaction(&tx,wallet.PrivateKey)
	return &tx
}

func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	if err != nil {
		log.Panic(err)
	}
	return transaction
}