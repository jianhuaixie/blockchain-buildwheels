package database

import (
	"github.com/blockchain-buildwheels/content/wheels-9/btclog"
	"fmt"
)

// 作为一个继承了DB接口的后端来注册
type Driver struct {
	DbType	string
	Create func(args ...interface{}) (DB,error)
	Open func(args ...interface{}) (DB,error)
	UseLogger func(logger btclog.Logger)
}

var drivers = make(map[string]*Driver)

// 将那些还没有注册的后端数据库驱动进行注册
func RegisterDriver(driver Driver) error{
	if _,exists := drivers[driver.DbType];exists{
		str := fmt.Sprintf("driver %q is already registered",
			driver.DbType)
		return makeError(ErrDbTypeRegistered,str,nil)
	}
	drivers[driver.DbType] = &driver
	return nil
}

// 返回能支持的数据库的一个slice
func SupportedDrivers() []string{
	supportedDBs := make([]string,0,len(drivers))
	for _,drv := range drivers{
		supportedDBs = append(supportedDBs,drv.DbType)
	}
	return supportedDBs
}

// 为特定类型的数据库创建初始化和打开一个数据库
func Create(dbType string,args ...interface{})(DB, error){
	drv , exists := drivers[dbType]
	if !exists{
		str := fmt.Sprintf("driver %q is not registered", dbType)
		return nil, makeError(ErrDbUnknownType, str, nil)
	}
	return drv.Create(args...)
}

// 为特定数据库类型打开一个已经存在的数据库
func Open(dbType string,args ...interface{}) (DB,error){
	drv,exists := drivers[dbType]
	if !exists {
		str := fmt.Sprintf("driver %q is not registered", dbType)
		return nil, makeError(ErrDbUnknownType, str, nil)
	}
	return drv.Open(args...)
}
