package main

import (
	"fmt"
	"os"
	"flag"
	"log"
	"strconv"
)

type CLI struct {}

func (cli *CLI) printUsage(){
	fmt.Println("Usage:")
	fmt.Println("  createblockchain -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
	fmt.Println("  createwallet - Generates a new key-pair and saves it into the wallet file")
	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
	fmt.Println("  printchain - Print all the blocks of the blockchain")
	fmt.Println("  reindexutxo - Rebuilds the UTXO set")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT -mine - Send AMOUNT of coins from FROM address to TO. Mine on the same node, when -mine is set.")
	fmt.Println("  startnode -miner ADDRESS - Start a node with ID specified in NODE_ID env. var. -miner enables mining")
}

func (cli *CLI) validateArgs(){
	if len(os.Args)<2{
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) Run(){
	cli.validateArgs()
	nodeID := os.Getenv("NODE_ID")
	if nodeID == ""{
		fmt.Printf("NODE_ID env. var is not set!")
		os.Exit(1)
	}
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)

	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	sendMine := sendCmd.Bool("mine", false, "Mine immediately on the same node")
	startNodeMiner := startNodeCmd.String("miner", "", "Enable mining mode and send reward to ADDRESS")

	switch os.Args[1] {
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createblockchain":
		err := createBlockchainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "listaddresses":
		err := listAddressesCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "reindexutxo":
		err := reindexUTXOCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "startnode":
		err := startNodeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		cli.printUsage()
		os.Exit(1)
	}

	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}
		cli.getBalance(*getBalanceAddress, nodeID)
	}

	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			os.Exit(1)
		}
		cli.createBlockchain(*createBlockchainAddress, nodeID)
	}

	if createWalletCmd.Parsed() {
		cli.createWallet(nodeID)
	}

	if listAddressesCmd.Parsed() {
		cli.listAddresses(nodeID)
	}

	if printChainCmd.Parsed() {
		cli.printChain(nodeID)
	}

	if reindexUTXOCmd.Parsed() {
		cli.reindexUTXO(nodeID)
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			os.Exit(1)
		}
		cli.send(*sendFrom, *sendTo, *sendAmount, nodeID, *sendMine)
	}

	if startNodeCmd.Parsed() {
		nodeID := os.Getenv("NODE_ID")
		if nodeID == "" {
			startNodeCmd.Usage()
			os.Exit(1)
		}
		cli.startNode(nodeID, *startNodeMiner)
	}
}

// 获取余额
func (cli *CLI) getBalance(address string, nodeID string) {
	// 第一步，验证地址是否合法
	if !ValidateAddress(address){
		log.Panic("ERROR: Address is not valid")
	}
	// 第二步，通过nodeID，构建区块链
	blockchain := NewBlockchain(nodeID)
	// 第三步，通过区块链，构建未花费交易输出集合
	UTXOSet := UTXOSet{blockchain}
	defer blockchain.db.Close()
	balance := 0
	// 第三步，根据address，找到为花费交易输出集合中属于该地址的集合
	pubKeyHash := Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-4]
	UTXOs := UTXOSet.FindUTXO(pubKeyHash)
	for _,out := range UTXOs{
		balance += out.Value
	}
	fmt.Printf("Balance of '%s': %d\n", address, balance)
}

// 用一个地址和nodeID创建一条链  包括两部分，一部分是构建blockchain，另一部分是构建utxo集合，都需要写入数据库
func (cli *CLI) createBlockchain(address string, nodeID string) {
	if !ValidateAddress(address){
		log.Panic("ERROR: Address is not valid")
	}
	blockchain := CreateBlockchain(address,nodeID)
	defer blockchain.db.Close()
	UTXOSet := UTXOSet{Blockchain:blockchain}
	UTXOSet.Reindex()
	fmt.Println("Done!")
}
func (cli *CLI) createWallet(nodeID string) {
	wallets,_ := NewWallets(nodeID)
	address := wallets.CreateWallet()
	wallets.SaveToFile(nodeID)
	fmt.Printf("Your new address: %s\n", address)
}
func (cli *CLI) listAddresses(nodeID string) {
	wallets,err := NewWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	addresses := wallets.GetAddresses()
	for _,address := range addresses{
		fmt.Println(address)
	}
}
func (cli *CLI) printChain(nodeID string) {
	blockchain := NewBlockchain(nodeID)
	defer blockchain.db.Close()
	blockchainiterator := blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		fmt.Printf("============ Block %x ============\n", block.Hash)
		fmt.Printf("Height: %d\n", block.Height)
		fmt.Printf("Prev. block: %x\n", block.PrevBlockHash)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n\n", strconv.FormatBool(pow.Validate()))
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		fmt.Printf("\n\n")
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
}
func (cli *CLI) reindexUTXO(nodeID string) {
	blockchain := NewBlockchain(nodeID)
	UTXOSet := UTXOSet{Blockchain:blockchain}
	UTXOSet.Reindex()
	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
}
func (cli *CLI) send(from, to string, amount int, nodeID string, mineNow bool) {
	if !ValidateAddress(from) {
		log.Panic("ERROR: Sender address is not valid")
	}
	if !ValidateAddress(to) {
		log.Panic("ERROR: Recipient address is not valid")
	}
	bc := NewBlockchain(nodeID)
	UTXOSet := UTXOSet{bc}
	defer bc.db.Close()
	wallets, err := NewWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)
	tx := NewUTXOTransaction(&wallet, to, amount, &UTXOSet)
	if mineNow {
		cbTx := NewCoinbaseTX(from, "")
		txs := []*Transaction{cbTx, tx}
		newBlock := bc.MineBlock(txs)
		UTXOSet.Update(newBlock)
	} else {
		sendTx(knownNodes[0], tx)
	}
	fmt.Println("Success!")
}
func (cli *CLI) startNode(nodeID string, minerAddress string) {
	fmt.Printf("Starting node %s\n", nodeID)
	if len(minerAddress)>0{
		if ValidateAddress(minerAddress){
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		}else{
			log.Panic("Wrong miner address!")
		}
	}
	StartServer(nodeID,minerAddress)
}

















