package main

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"io/ioutil"
	"log"
	"encoding/gob"
	"crypto/elliptic"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"golang.org/x/crypto/ripemd160"
)

const walletFile = "wallet_%s.dat"
const version = byte(0x00)
const addressChecksumLen = 4

type Wallet struct {
	PrivateKey	ecdsa.PrivateKey
	PublicKey	[]byte
}

// 从钱包中获取address
func (wallet *Wallet) GetAddress() []byte {
	pubKeyHash := HashPubKey(wallet.PublicKey)
	versionedPayload := append([]byte{version},pubKeyHash...)
	checksum := checksum(versionedPayload)
	fullPayload := append(versionedPayload,checksum...)
	address := Base58Encode(fullPayload)
	return address
}

// 生成一个checksum为公钥
func checksum(versionedPayload []byte) []byte {
	firstSHA := sha256.Sum256(versionedPayload)
	secondSHA := sha256.Sum256(firstSHA[:])
	return secondSHA[:addressChecksumLen]
}

// 将publickey进行hash操作
func HashPubKey(PublicKey []byte) []byte {
	publicSHA256 := sha256.Sum256(PublicKey)
	RIPEMD160Hasher := ripemd160.New()
	_,err := RIPEMD160Hasher.Write(publicSHA256[:])
	if err != nil {
		log.Panic(err)
	}
	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)
	return publicRIPEMD160
}

type Wallets struct{
	Wallets map[string]*Wallet
}

// 从本地文件中加载钱包地址
func (ws *Wallets) LoadFromFile(nodeID string) error {
	// 第一步，找到本地钱包地址，并且判断地址是否存在钱包
	walletFile := fmt.Sprintf(walletFile, nodeID)
	if _,err := os.Stat(walletFile);os.IsNotExist(err){
		return err
	}
	// 第二步，读取钱包地址中的数据
	fileContent,err := ioutil.ReadFile(walletFile)
	if err != nil {
		log.Panic(err)
	}
	// 第三步，构建钱包变量，将读取数据进行解码
	var wallets Wallets
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		log.Panic(err)
	}
	// 第四步，将钱包变量赋值给目标地址
	ws.Wallets = wallets.Wallets
	return nil
}
func (ws *Wallets) CreateWallet() string {
	wallet := NewWallet()
	address := fmt.Sprintf("%s", wallet.GetAddress())
	ws.Wallets[address] = wallet
	return address
}

// 将钱包写入到本地文件
func (wallets *Wallets) SaveToFile(nodeID string) {
	// 第一步，声明一个Buffer用来装钱包数据
	var content bytes.Buffer
	// 第二步，找到钱包存放目录
	walletFile := fmt.Sprintf(walletFile, nodeID)
	// 第三步，将数据进行编码处理
	gob.Register(elliptic.P256())
	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(wallets)
	if err != nil {
		log.Panic(err)
	}
	// 第四步，写入本地文件
	err = ioutil.WriteFile(walletFile,content.Bytes(),0644)
	if err != nil {
		log.Panic(err)
	}
}

// 将本地所有存的钱包地址取出来
func (ws *Wallets) GetAddresses() []string {
	var addresses []string
	for address := range ws.Wallets{
		addresses = append(addresses,address)
	}
	return addresses
}

// 从钱包集合中，通过地址找到钱包
func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

// 创建和返回一个钱包
func NewWallet() *Wallet {
	private,public := newKeyPair()
	wallet := Wallet{private,public}
	return &wallet
}
// 生成私钥公钥密码对
func newKeyPair() (ecdsa.PrivateKey,[]byte) {
	curve := elliptic.P256()
	private,err := ecdsa.GenerateKey(curve,rand.Reader)
	if err != nil {
		log.Panic(err)
	}
	pubKey := append(private.PublicKey.X.Bytes(),private.PublicKey.Y.Bytes()...)
	return *private,pubKey
}

// 从本地加载所有钱包地址
func NewWallets(nodeID string) (*Wallets,error){
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)
	err := wallets.LoadFromFile(nodeID)
	return &wallets, err
}

func ValidateAddress(address string) bool {
	pubKeyHash := Base58Decode([]byte(address))
	actualChecksum := pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]
	targetChecksum := checksum(append([]byte{version}, pubKeyHash...))
	return bytes.Compare(actualChecksum, targetChecksum) == 0
}