package logtool

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"path"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var inited bool

type Writer interface {
	write(Level, string)
	exit()
}

type Level byte

func (l *Level) Set(s string) error {
	for k, v := range levelName {
		level := Level(k)
		if level != levelDefault && v == s {
			*l = level
			return nil
		}
	}
	return errors.New("invaild level")
}

func (l *Level) String() string {
	return levelName[*l]
}

const (
	levelDefault Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelAction
)

var levelName = []string{
	levelDefault: "output",
	LevelDebug:   "debug",
	LevelInfo:    "info",
	LevelWarn:    "warn",
	LevelError:   "error",
	LevelAction:  "action",
}

var (
	flagAlseStdout bool
	flagLoglevel   Level
	flagLogname    string
	flagLogpath    string
)

func init() {
	flag.BoolVar(&flagAlseStdout, "alsostdout", true, "log to standard error as well as files")
	flag.Var(&flagLoglevel, "loglevel", "log level[debug,info,warn,error]")
	flag.StringVar(&flagLogname, "logname", "", "log name")
	flag.StringVar(&flagLogpath, "logpath", "", "log path default for filelog(/data/)")
	log.SetFlags(1)
	log.SetOutput(NewLogWriter(levelDefault))
}

var (
	logPath    = "/data"
	logName    = "logtool"
	logLevel   = LevelDebug
	alsoStdout = false
	logWriter  Writer
	mu         sync.Mutex
	skip       = 3
)

func level() Level {
	if flagLoglevel > levelDefault {
		return flagLoglevel
	}
	return logLevel
}

func initWriter() {
	mu.Lock()
	defer mu.Unlock()
	if logWriter != nil {
		return
	}
	if flagLogname != "" {
		logName = flagLogname
	}
	if flagLogpath != "" {
		logPath = flagLogpath
	}
	logWriter = newFileLog(logName, logPath)
}

func Init(logName string, logLevel Level, stdOut bool) {
	if inited {
		fmt.Println("logtool has be inited")
	}
	alsoStdout = stdOut
	SetName(logName)
	SetLevel(logLevel)
	inited = true
	//kafkaInit()
}

func SetLevel(level Level) {
	if level < LevelDebug || level > LevelError {
		panic("invalid log level")
	}
	logLevel = level
}

func SetName(name string) {
	if name == "" {
		panic("invalid log name")
	}
	logName = name
}

func AlsoStdout(b bool) {
	alsoStdout = b
}

func SetLogPath(path string) {
	if path != "" {
		logPath = path
	}
}

func write(level Level, msg string) {
	if !inited {
		fmt.Println(time.Now().Format("2006-01-02 15:04:05 ") + GetPrefix(skip) + " [" + levelName[level] + "] " + msg)
		return
	}
	if logWriter == nil {
		initWriter()
	}
	logWriter.write(level, msg)
	if alsoStdout {
		fmt.Println(time.Now().Format("2006-01-02 15:04:05") + " [" + levelName[level] + "] " + msg)
	}
}

func SetSkip(s int) {
	skip = s
}

var bfPool sync.Pool

func init() {
	bfPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
}

func GetPrefix(skip int) string {
	_, file, line, ok := runtime.Caller(skip)

	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	buf := bfPool.Get().(*bytes.Buffer)
	buf.WriteString(fmt.Sprintf("reqid-%d ", n))
	if ok {
		_, filename := path.Split(file)
		buf.WriteString(filename)
		buf.WriteString(" ")
		buf.WriteString(strconv.Itoa(line))
		buf.WriteString(" : ")
		s := buf.String()
		return s
	} else {
		buf.WriteString(" ??? ")
		s := buf.String()
		return s
	}
	return ""
}

func Exit() {
	if logWriter != nil {
		logWriter.exit()
	}
}

func IsDebug() bool {
	return level() == LevelDebug
}

func Debug(str string) {
	if level() <= LevelDebug {
		write(LevelDebug, str)
	}
}

func Debugs(args ...interface{}) {
	if level() <= LevelDebug {
		write(LevelDebug, fmt.Sprintln(args...))
	}
}

func Debugf(format string, args ...interface{}) {
	if level() <= LevelDebug {
		write(LevelDebug, fmt.Sprintf(format, args...))
	}
}

func Info(str string) {
	if level() <= LevelInfo {
		write(LevelInfo, str)
	}
}

func Infos(args ...interface{}) {
	if level() <= LevelInfo {
		write(LevelInfo, fmt.Sprintln(args...))
	}
}

func Infof(format string, args ...interface{}) {
	if level() <= LevelInfo {
		write(LevelInfo, fmt.Sprintf(format, args...))
	}
}

func Warn(str string) {
	if level() <= LevelWarn {
		write(LevelWarn, str)
	}
}

func Warns(args ...interface{}) {
	if level() <= LevelWarn {
		write(LevelWarn, fmt.Sprintln(args...))
	}
}

func Warnf(format string, args ...interface{}) {
	if level() <= LevelWarn {
		write(LevelWarn, fmt.Sprintf(format, args...))
	}
}

func Error(str string) {
	if level() <= LevelError {
		write(LevelError, str)
	}
}

func Errors(args ...interface{}) {
	if level() <= LevelError {
		write(LevelError, fmt.Sprintln(args...))
	}
}

func Errorf(format string, args ...interface{}) {
	if level() <= LevelError {
		write(LevelError, fmt.Sprintf(format, args...))
	}
}

func Action(v interface{}) error {
	str, err := json.Marshal(v)
	if err != nil {
		return errors.New("action data is empty")
	}
	write(LevelAction, string(str))
	return nil
}

type Fields map[string]interface{}

func NewStdLog(level Level, prefix string) *log.Logger {
	return log.New(NewLogWriter(level), prefix, 0)
}

func NewLogWriter(level Level) io.Writer {
	return &LogWriter{level}
}

type LogWriter struct {
	level Level
}

func (this *LogWriter) Write(data []byte) (int, error) {
	l := this.level
	if this.level == levelDefault && len(data) > 3 && data[0] == '[' { // from built-in log
		switch string(data[:3]) {
		case "[D]":
			l = LevelDebug
			data = data[3:]
		case "[I]":
			l = LevelInfo
			data = data[3:]
		case "[W]":
			l = LevelWarn
			data = data[3:]
		case "[E]":
			l = LevelError
			data = data[3:]
		}
	}

	if l == levelDefault || level() <= l {
		write(l, string(data))
	}
	return len(data), nil
}
