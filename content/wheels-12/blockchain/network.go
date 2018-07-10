package blockchain

import (
	"bytes"
	"encoding/gob"
	"log"
	"fmt"
	"net"
	"io"
	"time"
	"io/ioutil"
)

const protocol = "tcp"
const nodeVersion = 1
const commandLength = 12

var knownNodes = []string{"localhost:3000"}
var nodeAddress string

type block struct {
	AddrFrom string
	Block []byte
}
type delegateSt struct {
	AddrFrom string
	Delegates []byte
}

func gobEncode(data interface{}) []byte{
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func commandToBytes(command string) []byte{
	var bytes [commandLength]byte
	for i,c := range command{
		bytes[i] = byte(c)
	}
	return bytes[:]
}
func bytesToCommand(bytes []byte) string{
	var command []byte
	for _,b := range bytes{
		if b != 0x0{
			command = append(command,b)
		}
	}
	return fmt.Sprintf("%s",command)
}

func sendDelegates(blockchain *Blockchain,numberDelegate int,delegate *Delegates){
	listDelegates := GetDelegates(blockchain)
	for _,tmpDelegate := range listDelegates{
		log.Println(tmpDelegate.Address, nodeAddress, numberDelegate, delegate.NumPeer)
		if tmpDelegate.Address != nodeAddress && numberDelegate>delegate.NumPeer{
			data := delegateSt{nodeAddress,delegate.SerializeDelegate()}
			payload := gobEncode(data)
			request := append(commandToBytes("delegates"),payload...)
			sendData(tmpDelegate.Address,request)
		}

		if tmpDelegate.Address != nodeAddress{
			data := delegateSt{tmpDelegate.Address,tmpDelegate.SerializeDelegate()}
			payload := gobEncode(data)
			request := append(commandToBytes("delegates"),payload...)
			sendData(delegate.Address,request)
		}
	}
}

func SendBlock(address string,b *Block){
	data := block{nodeAddress,b.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes("block"),payload...)
	sendData(address,request)
}

func sendData(address string,data []byte)  {
	conn,err := net.Dial(protocol,address)
	defer conn.Close()
	_,err = io.Copy(conn,bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
	}
}

// starts a node
func StartServer(nodeID string){
	nodeAddress = fmt.Sprintf("localhost:%s",nodeID)
	log.Println("Peer start address: ",nodeAddress)
	listen,err := net.Listen(protocol,nodeAddress)
	if err != nil{
		log.Panic(err)
	}
	defer listen.Close()
	blockchain := NewBlockchain(nodeID)
	lastHeight := blockchain.GetBestHeight()
	numberDelegate := GetNumberDelegates(blockchain)
	delegate := &Delegates{nodeVersion,lastHeight,nodeAddress,numberDelegate}
	InsertDelegates(blockchain,delegate,lastHeight)
	if nodeAddress != knownNodes[0]{
		sendDelegates(blockchain,numberDelegate+1,delegate)
	}
	go Forks(blockchain)
	for {
		conn,err := listen.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleConnection(conn,blockchain)
	}
}

func handleConnection(conn net.Conn,blockchain *Blockchain){
	time.Sleep(time.Second)
	request,err := ioutil.ReadAll(conn)
	if err != nil {
		log.Panic(err)
	}
	command := bytesToCommand(request[:commandLength])
	log.Printf("Received %s command\n", command)
	switch command {
	case "block":
		handleBlock(request,blockchain)
	case "delegates":
		handleDelegates(request,blockchain)
	default:
		fmt.Println("Unknown cmmand")
	}
	conn.Close()
}

func handleBlock(request []byte,blockchain *Blockchain){
	var buff bytes.Buffer
	var payload block
	buff.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buff)
	err := decoder.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	blockData := payload.Block
	block := DeserializeBlock(blockData)
	log.Println("Recevied a new block!")
	blockchain.AddBlock(block)
	log.Println("Added block", block.Hash)
	listPeer := GetDelegates(blockchain)
	for _,peer := range listPeer{
		UpdateDelegate(blockchain,peer.Address,block.Height)
	}
}

func handleDelegates(request []byte,blockchain *Blockchain){
	var buff bytes.Buffer
	var payload delegateSt
	buff.Write(request[commandLength:])
	decoder := gob.NewDecoder(&buff)
	err := decoder.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	delegateData := payload.Delegates
	delegate := DeserializePeer(delegateData)
	myBestHeight := blockchain.GetBestHeight()
	// insert delegates
	isInsert := InsertDelegates(blockchain,delegate,myBestHeight)
	log.Println("handleDelegate", payload.AddrFrom, delegate, myBestHeight, isInsert)
	numberDelegates := GetNumberDelegates(blockchain)
	log.Println(numberDelegates, delegate.NumPeer)
	if isInsert {
		sendDelegates(blockchain, numberDelegates, delegate)
	}
}