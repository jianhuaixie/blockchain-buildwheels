package btclog

type Logger interface {
	Tracef(format string,params ...interface{})
	Debugf(format string,params ...interface{})
	Infof(format string, params ...interface{})
	Warnf(format string, params ...interface{})
	Errorf(format string, params ...interface{})
	Criticalf(format string, params ...interface{})

	Trace(v ...interface{})
	Debug(v ...interface{})
	Info(v ...interface{})
	Warn(v ...interface{})
	Error(v ...interface{})
	Critical(v ...interface{})
	Level() Level
	SetLevel(level Level)
}
