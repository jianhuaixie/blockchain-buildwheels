package main

import (
	"github.com/btcsuite/btcutil"
	"github.com/blockchain-buildwheels/content/wheels-8/database"
	"github.com/btcsuite/btcd/chaincfg"
	"path/filepath"
	"os"
	"github.com/btcsuite/btcd/wire"
	"fmt"
	"errors"
)

var (
	btcdHomeDir = btcutil.AppDataDir("btcd",false)
	knownDbTypes = database.SupportedDrivers()
	activeNetParams = &chaincfg.MainNetParams
	cfg = &config{
		DataDir:filepath.Join(btcdHomeDir,"data"),
		DbType:"ffldb",
	}
)

type config struct {
	DataDir        string `short:"b" long:"datadir" description:"Location of the btcd data directory"`
	DbType         string `long:"dbtype" description:"Database backend to use for the Block Chain"`
	TestNet3       bool   `long:"testnet" description:"Use the test network"`
	RegressionTest bool   `long:"regtest" description:"Use the regression test network"`
	SimNet         bool   `long:"simnet" description:"Use the simulation test network"`
}

func fileExists(name string) bool{
	if _,err := os.Stat(name);err != nil{
		if os.IsNotExist(err){
			return false
		}
	}
	return true
}

func validDbType(dbType string) bool{
	for _,knownType := range knownDbTypes{
		if dbType == knownType{
			return true
		}
	}
	return false
}

func netName(chainParams *chaincfg.Params) string{
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}

func setupGlobalConfig() error{
	numNets := 0
	if cfg.TestNet3{
		numNets++
		activeNetParams = &chaincfg.TestNet3Params
	}
	if cfg.RegressionTest {
		numNets++
		activeNetParams = &chaincfg.RegressionNetParams
	}
	if cfg.SimNet {
		numNets++
		activeNetParams = &chaincfg.SimNetParams
	}
	if numNets > 1 {
		return errors.New("The testnet, regtest, and simnet params " +
			"can't be used together -- choose one of the three")
	}
	if !validDbType(cfg.DbType) {
		str := "The specified database type [%v] is invalid -- " +
			"supported types %v"
		return fmt.Errorf(str, cfg.DbType, knownDbTypes)
	}
	cfg.DataDir = filepath.Join(cfg.DataDir,netName(activeNetParams))
	return nil
}