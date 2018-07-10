package btclog

import (
	"strings"
	"os"
	"io"
	"sync"
	"time"
	"runtime"
	"bytes"
	"fmt"
	"sync/atomic"
	"io/ioutil"
)

var defaultFlags uint32

const (
	Llongfile uint32 = 1 << iota
	Lshortfile
)
func init(){
	// 从LOGFLAGS环境变量读取logger标志。多个信号可以立即设置，用逗号隔开。
	for _, f := range strings.Split(os.Getenv("LOGFLAGS"), ",") {
		switch f {
		case "longfile":
			defaultFlags |= Llongfile
		case "shortfile":
			defaultFlags |= Lshortfile
		}
	}
}

type Level uint32

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
	LevelOff
)

// 声明一个人类易读的字符串数组
var levelStrs = [...]string{"TRC", "DBG", "INF", "WRN", "ERR", "CRT", "OFF"}

// 根据输入字符串s，返回log等级level
func LevelFromString(s string)(l Level,ok bool){
	switch strings.ToLower(s) {
	case "trace", "trc":
		return LevelTrace, true
	case "debug", "dbg":
		return LevelDebug, true
	case "info", "inf":
		return LevelInfo, true
	case "warn", "wrn":
		return LevelWarn, true
	case "error", "err":
		return LevelError, true
	case "critical", "crt":
		return LevelCritical, true
	case "off":
		return LevelOff, true
	default:
		return LevelInfo, false
	}
}

// 返回logger的标记用于log messages
func (l Level) String() string{
	if l >= LevelOff {
		return "OFF"
	}
	return levelStrs[l]
}

// logging后端，后端给所有子系统提供原子化的写操作
type Backend struct{
	w	io.Writer
	mu	sync.Mutex  // ensures aotmic writes
	flag	uint32
}

// BackendOption是一个用来改变后端行为方式的方法
type BackendOption func(b *Backend)

// 从写操作创建一个日志后端
func NewBackend(w io.Writer,opts ...BackendOption) *Backend{
	backend := &Backend{w:w,flag:defaultFlags}
	for _,o := range opts{
		o(backend)
	}
	return backend
}

// 给后端配置上写的flags
func WithFlags(flags uint32) BackendOption{
	return func(b *Backend) {
		b.flag = flags
	}
}

// 缓冲池定义了一个并发安全的字节片列表，用于在输出日志消息之前为格式化日志消息提供临时缓冲。
var bufferPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte,0,120)
		return &buffer
	},
}

// 缓冲区从空闲列表中返回一个字节片。如果空闲列表中没有可用的，则分配一个新的缓冲区。返回的字节片应该在调用者完成后，通过使用recycleBuffer函数返回空闲列表。
func buffer() *[]byte{
	return bufferPool.Get().(*[]byte)
}

// 将所提供的字节片放在空闲列表上，这应该是通过缓冲函数获得的。
func recycleBuffer(buffer *[]byte){
	*buffer = (*buffer)[:0]
	bufferPool.Put(buffer)
}

func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func formatHeader(buf *[]byte, t time.Time, lvl, tag string, file string, line int) {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	ms := t.Nanosecond() / 1e6

	itoa(buf, year, 4)
	*buf = append(*buf, '-')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '-')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, min, 2)
	*buf = append(*buf, ':')
	itoa(buf, sec, 2)
	*buf = append(*buf, '.')
	itoa(buf, ms, 3)
	*buf = append(*buf, " ["...)
	*buf = append(*buf, lvl...)
	*buf = append(*buf, "] "...)
	*buf = append(*buf, tag...)
	if file != "" {
		*buf = append(*buf, ' ')
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
	}
	*buf = append(*buf, ": "...)
}

const calldepth = 3

// 将callsite的文件名和行号返回到子系统日志记录器。
func callsite(flag uint32) (string ,int){
	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		return "???", 0
	}
	if flag&Lshortfile != 0 {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if os.IsPathSeparator(file[i]) {
				short = file[i+1:]
				break
			}
		}
		file = short
	}
	return file, line
}

// 将一个日志消息输出到与后端相关的写入器，因为它为给定的级别创建了一个前缀，并根据formader函数进行标记，并使用缺省格式化规则格式化所提供的参数。
func (b *Backend) print(lvl, tag string, args ...interface{}) {
	t := time.Now() // get as early as possible
	bytebuf := buffer()
	var file string
	var line int
	if b.flag&(Lshortfile|Llongfile) != 0 {
		file, line = callsite(b.flag)
	}

	formatHeader(bytebuf, t, lvl, tag, file, line)
	buf := bytes.NewBuffer(*bytebuf)
	fmt.Fprintln(buf, args...)
	*bytebuf = buf.Bytes()

	b.mu.Lock()
	b.w.Write(*bytebuf)
	b.mu.Unlock()

	recycleBuffer(bytebuf)
}

func (b *Backend) printf(lvl, tag string, format string, args ...interface{}) {
	t := time.Now() // get as early as possible
	bytebuf := buffer()
	var file string
	var line int
	if b.flag&(Lshortfile|Llongfile) != 0 {
		file, line = callsite(b.flag)
	}

	formatHeader(bytebuf, t, lvl, tag, file, line)
	buf := bytes.NewBuffer(*bytebuf)
	fmt.Fprintf(buf, format, args...)
	*bytebuf = append(buf.Bytes(), '\n')

	b.mu.Lock()
	b.w.Write(*bytebuf)
	b.mu.Unlock()

	recycleBuffer(bytebuf)
}

// slog是后端的子系统日志记录器。实现了日志接口。
type slog struct {
	lvl  Level
	tag string
	b *Backend
}

func (b *Backend) Logger(subsystemTag string) Logger {
	return &slog{LevelInfo, subsystemTag, b}
}

func (l *slog) Trace(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelTrace {
		l.b.print("TRC", l.tag, args...)
	}
}

func (l *slog) Tracef(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelTrace {
		l.b.printf("TRC", l.tag, format, args...)
	}
}

func (l *slog) Debug(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelDebug {
		l.b.print("DBG", l.tag, args...)
	}
}

func (l *slog) Debugf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelDebug {
		l.b.printf("DBG", l.tag, format, args...)
	}
}

func (l *slog) Info(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelInfo {
		l.b.print("INF", l.tag, args...)
	}
}

func (l *slog) Infof(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelInfo {
		l.b.printf("INF", l.tag, format, args...)
	}
}

func (l *slog) Warn(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelWarn {
		l.b.print("WRN", l.tag, args...)
	}
}

func (l *slog) Warnf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelWarn {
		l.b.printf("WRN", l.tag, format, args...)
	}
}

func (l *slog) Error(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelError {
		l.b.print("ERR", l.tag, args...)
	}
}

func (l *slog) Errorf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelError {
		l.b.printf("ERR", l.tag, format, args...)
	}
}

func (l *slog) Critical(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelCritical {
		l.b.print("CRT", l.tag, args...)
	}
}

func (l *slog) Criticalf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelCritical {
		l.b.printf("CRT", l.tag, format, args...)
	}
}

func (l *slog) Level() Level {
	return Level(atomic.LoadUint32((*uint32)(&l.lvl)))
}

func (l *slog) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&l.lvl), uint32(level))
}

var Disabled Logger

func init() {
	Disabled = &slog{lvl: LevelOff, b: NewBackend(ioutil.Discard)}
}