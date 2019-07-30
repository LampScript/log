package logtool

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	MaxSize       uint64 = 1024 * 1024 * 1800
	bufferSize           = 256 * 1024
	flushInterval        = 5 * time.Second
)

type fileLogWriter struct {
	basePath   string
	logName    string
	writers    [6]*bufferWriter
	mu         sync.Mutex
	freeList   *buffer
	freeListMu sync.Mutex
	bufPool    sync.Pool
}

func newFileLog(logName, basePath string) *fileLogWriter {
	writer := &fileLogWriter{
		basePath: basePath,
		logName:  logName,
		bufPool:  sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
	}
	go writer.flushDaemon()
	return writer
}

func (w *fileLogWriter) exit() {
	w.flushAll()
}

func (w *fileLogWriter) flushDaemon() {
	for _ = range time.NewTicker(flushInterval).C {
		w.flushAll()
	}
}

func (w *fileLogWriter) flushAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, writer := range w.writers {
		if writer != nil && writer.Writer != nil {
			writer.Flush()
			writer.Sync()
		}
	}
}

type buffer struct {
	bytes.Buffer
	next *buffer
}

func (w *fileLogWriter) getBuffer() *buffer {
	w.freeListMu.Lock()
	b := w.freeList
	if b != nil {
		w.freeList = b.next
	}
	w.freeListMu.Unlock()
	if b == nil {
		b = new(buffer)
	} else {
		b.next = nil
		b.Reset()
	}
	return b
}

func (w *fileLogWriter) putBuffer(b *buffer) {
	if b.Len() >= 256 {
		// Let big buffers die with gc.
		return
	}
	w.freeListMu.Lock()
	b.next = w.freeList
	w.freeList = b
	w.freeListMu.Unlock()
}

func (w *fileLogWriter) write(level Level, s string) {
	now := time.Now()
	buf := bfPool.Get().(*bytes.Buffer)
	if level != LevelAction {
		timestamp := now.Format("2006-01-02 15:04:05.999 ")
		buf.WriteString(timestamp)
	}
	buf.WriteString(GetPrefix(4))
	buf.WriteString(s)
	if buf.Bytes()[buf.Len()-1] != '\n' {
		buf.WriteByte('\n')
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	writer := w.writers[level]
	if writer == nil {
		writer = &bufferWriter{
			basePath: w.basePath,
			logName:  w.logName,
			level:    level,
		}
		w.writers[level] = writer
	}
	if err := writer.checkRotate(now); err != nil {
		fmt.Println("[logtool] check rotate err: " + err.Error())
		return
	}
	writer.Write(buf.Bytes())
}

type bufferWriter struct {
	*bufio.Writer
	basePath string
	logName  string
	file     *os.File
	level    Level
	stime    time.Time
	slot     int
	nbytes   uint64 // The number of bytes written to this file
}

func (sb *bufferWriter) Sync() error {
	return sb.file.Sync()
}

func (sb *bufferWriter) Write(p []byte) (int, error) {
	n, err := sb.Writer.Write(p)
	sb.nbytes += uint64(n)
	return n, err
}

func (sb *bufferWriter) checkRotate(now time.Time) error {
	if sb.file == nil {
		return sb.rotateFile(now, 0)
	}
	syear, smonth, sday := sb.stime.Date()
	year, month, day := now.Date()
	if year != syear || month != smonth || day != sday {
		return sb.rotateFile(now, 0)
	}
	hour := now.Hour()
	shour := sb.stime.Hour()
	if hour != shour {
		return sb.rotateFile(now, 0)
	}
	if sb.nbytes >= MaxSize {
		return sb.rotateFile(now, sb.slot+1)
	}
	return nil
}

func (sb *bufferWriter) rotateFile(now time.Time, slot int) error {
	if sb.file != nil {
		sb.Flush()
		sb.file.Close()
	}
	var err error
	file, err := createFile(sb.basePath, sb.logName, sb.level, slot, now)
	if err != nil {
		return err
	}
	sb.file = file
	sb.nbytes = 0
	sb.stime = now
	sb.slot = slot
	if err != nil {
		return err
	}
	sb.Writer = bufio.NewWriterSize(sb.file, bufferSize)
	return err
}

func createFile(basePath, logName string, level Level, slot int, t time.Time) (*os.File, error) {
	year, month, day := t.Date()
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	basePath += "logs"
	logDir := filepath.Join(basePath, fmt.Sprintf("%s/%04d%02d/%02d/", logName, year, month, day))
	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("logtool: cannot create log: %v", err)
	}
	var logFile string
	if slot <= 0 {
		logFile = fmt.Sprintf("%s-%02d.log", levelName[level], t.Hour())
	} else {
		logFile = fmt.Sprintf("%s-%02d-%d.log", levelName[level], t.Hour(), slot)
	}
	fname := filepath.Join(logDir, logFile)
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("logtool: cannot open log file: %v", err)
	}
	return f, nil
}
