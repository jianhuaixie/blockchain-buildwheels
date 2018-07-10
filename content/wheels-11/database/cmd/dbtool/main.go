package main

import (
	"github.com/blockchain-buildwheels/content/wheels-9/btclog"
	"github.com/blockchain-buildwheels/content/wheels-9/database"
	"path/filepath"
	"os"
	"runtime"
	"strings"
	flags "github.com/jessevdk/go-flags"
)

const (
	blockDbNamePrefix = "blocks"
)

var (
	log	btclog.Logger
	shutdownChannel	= make(chan error)
)

// 打开区块数据库，然后可以去操作
func loadBlockDB()(database.DB,error){
	dbName := blockDbNamePrefix + "_" + cfg.DbType
	dbPath := filepath.Join(cfg.DataDir,dbName)
	log.Infof("Loading block database from '%s'", dbPath)
	db,err := database.Open(cfg.DbType,dbName)
	if err != nil{
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {
			return nil, err
		}
		err = os.MkdirAll(cfg.DataDir,0700)
		if err != nil {
			return nil, err
		}
		db,err = database.Create(cfg.DbType,dbPath,activeNetParams.Net)
		if err != nil {
			return nil, err
		}
	}
	log.Info("Block database loaded")
	return db,nil
}

// 设置解析的参数和命令，然后启动命令执行，其中包括四个命令：insecureimport，loadheaders，fetchblock，fetchblockregion
func realMain() error{
	// Setup logging.
	backendLogger := btclog.NewBackend(os.Stdout)
	defer os.Stdout.Sync()
	log = backendLogger.Logger("MAIN")
	dbLog := backendLogger.Logger("BCDB")
	dbLog.SetLevel(btclog.LevelDebug)
	database.UseLogger(dbLog)

	// Setup the parser options and commands.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	parserFlags := flags.Options(flags.HelpFlag | flags.PassDoubleDash)
	parser := flags.NewNamedParser(appName, parserFlags)
	parser.AddGroup("Global Options", "", cfg)
	parser.AddCommand("insecureimport",
		"Insecurely import bulk block data from bootstrap.dat",
		"Insecurely import bulk block data from bootstrap.dat.  "+
			"WARNING: This is NOT secure because it does NOT "+
			"verify chain rules.  It is only provided for testing "+
			"purposes.", &importCfg)
	parser.AddCommand("loadheaders",
		"Time how long to load headers for all blocks in the database",
		"", &headersCfg)
	parser.AddCommand("fetchblock",
		"Fetch the specific block hash from the database", "",
		&fetchBlockCfg)
	parser.AddCommand("fetchblockregion",
		"Fetch the specified block region from the database", "",
		&blockRegionCfg)

	// Parse command line and invoke the Execute function for the specified command.
	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		} else {
			log.Error(err)
		}
		return err
	}
	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	if err := realMain();err != nil{
		os.Exit(1)
	}
}