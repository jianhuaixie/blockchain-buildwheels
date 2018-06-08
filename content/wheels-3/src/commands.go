package main

import (
	"fmt"
	"strconv"
)

func (cli *CLI) addBlock(data string){
	cli.blockchain.AddBlock(data)
	fmt.Println("add block success!")
}
func (cli *CLI) printChain(){
	blockchainiterator := cli.blockchain.Iterator()
	for {
		block := blockchainiterator.Next()
		fmt.Printf("Prev hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()
		if len(block.PrevBlockHash) == 0{
			break
		}
	}
}
