package database

import "github.com/blockchain-buildwheels/content/wheels-9/btclog"

var log btclog.Logger

func init(){
	DisableLog()
}

func DisableLog(){
	log = btclog.Disabled
}

func UseLogger(logger btclog.Logger){
	log = logger
	for _, drv := range drivers {
		if drv.UseLogger != nil {
			drv.UseLogger(logger)
		}
	}
}