package main

func main()  {
	blockchain := NewBlockchain()
	defer blockchain.db.Close()
	cli := CLI{blockchain}
	cli.Run()
}
