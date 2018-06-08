package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"
	"crypto/sha256"
	"golang.org/x/crypto/ripemd160"
	"bytes"
	"os"
	"io/ioutil"
	"encoding/gob"
	"fmt"
)

const version = byte(0x00)
const walletFile = "wallet.dat"
const addressChecksumLen = 4

// Wallet stores privatte and public keys
type Wallet struct {
	PrivateKey	ecdsa.PrivateKey
	PublicKey	[]byte
}
type Wallets struct {
	Wallets map[string]*Wallet
}

// creates Wallets and fills it form a file f it exists
func NewWallets() (*Wallets,error){
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)
	err := wallets.LoadFromFile()
	return &wallets,err
}

// adds a Wallet to Wallets
func (ws *Wallets) CreateWallet() string{
	wallet := NewWallet()
	address := fmt.Sprintf("%s",wallet.GetAddress())
	ws.Wallets[address] = wallet
	return address
}

// return s an array of addresses stored in the wallet file
func (ws *Wallets) GetAddresses() []string{
	var addresses []string
	for address := range ws.Wallets{
		addresses = append(addresses,address)
	}
	return addresses
}

// returns a Wallet by its address
func (ws Wallets) GetWallet(address string) Wallet{
	return *ws.Wallets[address]
}



// loads wallets from the file
func (ws *Wallets) LoadFromFile() error{
	if _,err := os.Stat(walletFile);os.IsNotExist(err){
		return err
	}
	fileContent,err := ioutil.ReadFile(walletFile)
	if err != nil {
		log.Panic(err)
	}
	var wallets Wallets
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		log.Panic(err)
	}
	ws.Wallets = wallets.Wallets
	return nil
}

// saves wallets to a file
func (ws Wallets) SaveToFile(){
	var content bytes.Buffer
	gob.Register(elliptic.P256())
	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	if err != nil {
		log.Panic(err)
	}
	err = ioutil.WriteFile(walletFile,content.Bytes(),0644)
	if err != nil {
		log.Panic(err)
	}
}

func newKeyPair() (ecdsa.PrivateKey,[]byte){
	curve := elliptic.P256()
	private,err := ecdsa.GenerateKey(curve,rand.Reader)
	if err!=nil{
		log.Panic(err)
	}
	pubKey := append(private.PublicKey.X.Bytes(),private.PublicKey.Y.Bytes()...)
	return *private,pubKey
}
func NewWallet() *Wallet{
	private,public := newKeyPair()
	wallet := Wallet{private,public}
	return &wallet
}
func HashPubKey(pubKey []byte) []byte{
	publicSHA256 := sha256.Sum256(pubKey)
	RIPEMD160Hasher := ripemd160.New()
	_,err := RIPEMD160Hasher.Write(publicSHA256[:])
	if err != nil {
		log.Panic(err)
	}
	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)
	return publicRIPEMD160
}
// Checksum generates a checksum for a public key
func checksum(payload []byte) []byte{
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])
	return secondSHA[:addressChecksumLen]
}
func (wallet Wallet) GetAddress() []byte{
	pubKeyHash := HashPubKey(wallet.PublicKey)
	versionedPayload := append([]byte{version},pubKeyHash...)
	checksum := checksum(versionedPayload)
	fullPayload := append(versionedPayload,checksum...)
	address := Base58Encode(fullPayload)
	return address
}
// validateAddress check if address if valid
func ValidateAddress(address string) bool{
	pubKeyHash := Base58Decode([]byte(address))
	actualChecksum := pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-addressChecksumLen]
	targetChecksum := checksum(append([]byte{version},pubKeyHash...))
	return bytes.Compare(actualChecksum,targetChecksum)==0
}