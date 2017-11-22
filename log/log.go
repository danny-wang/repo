package log

import (
	"encoding/json"
	"fmt"
	"io"
	stdLog "log"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DebugLevel int

const (
	DEBUG DebugLevel = 1 << iota
	INFO
	WARN
	ERROR
	PANIC
	FATAL
)

type VerboseLevel int

const (
	NORMAL VerboseLevel = 1 << iota
	MORE
	MUCH
)

const (
	RED    = "31"
	GREEN  = "32"
	YELLOW = "33"
	BLUE   = "34"
	PINK   = "35"
	CYAN   = "36"
)

var Level = DEBUG
var Verbose = NORMAL
var IsTerminal = true

type Logger struct {
	depth   int
	verbose VerboseLevel
	reqid   string
	Logger  *stdLog.Logger
}

func NewLogger(l int) *Logger {
	return &Logger{l, MUCH, "", stdGoLog}
}

func NewLoggerEx(depth int, verbose VerboseLevel, reqid string) *Logger {
	switch verbose {
	case MUCH:
		Level = DEBUG
	case MORE:
		Level = DEBUG
	case NORMAL:
		Level = INFO
	}
	Verbose = verbose
	return &Logger{1, verbose, reqid, NewGoLog(os.Stderr)}
}

func NewGoLog(w io.Writer) *stdLog.Logger {
	return stdLog.New(w, "", stdLog.LstdFlags)
}

var stdGoLog = stdLog.New(os.Stderr, "", stdLog.LstdFlags)
var std = NewLogger(1)
var (
	Println = std.Println
	Printf  = std.Printf
	Debug   = std.Debug
	Debugf  = std.Debugf
	Infof   = std.Infof
	Info    = std.Info
	Warn    = std.Warn
	Warnf   = std.Warnf
	Error   = std.Error
	Errorf  = std.Errorf
	Panic   = std.Panic
	Panicf  = std.Panicf
	Fatal   = std.Fatal
	Fatalf  = std.Fatalf

	SetReqId   = std.SetReqId
	SetOutput  = std.SetOutput
	PrintStack = std.PrintStack
	Stack      = std.Stack
	Struct     = std.Struct
	Pretty     = std.Pretty
	Todo       = std.Todo
)

func SetStd(l *Logger) {
	std = l
	Println = std.Println
	Printf = std.Printf
	Debug = std.Debug
	Debugf = std.Debugf
	Infof = std.Infof
	Info = std.Info
	Warn = std.Warn
	Warnf = std.Warnf
	Error = std.Error
	Errorf = std.Errorf
	Panic = std.Panic
	Panicf = std.Panicf
	Fatal = std.Fatal
	Fatalf = std.Fatalf

	SetReqId = std.SetReqId
	PrintStack = std.PrintStack
	Stack = std.Stack
	Struct = std.Struct
	Pretty = std.Pretty
	Todo = std.Todo
}

func color(col, s string) string {
	if col == "" {
		return s
	}
	return "\x1b[0;" + col + "m" + s + "\x1b[0m"
}
func setTerminalColor(s, col string) string {
	if IsTerminal {
		return color(col, s)
	}
	return s
}

func Red(s string) string {
	return setTerminalColor(s, RED)
}
func Green(s string) string {
	return setTerminalColor(s, GREEN)
}
func Yellow(s string) string {
	return setTerminalColor(s, YELLOW)
}
func Pink(s string) string {
	return setTerminalColor(s, PINK)
}
func Blue(s string) string {
	return setTerminalColor(s, BLUE)
}
func Cyan(s string) string {
	return setTerminalColor(s, CYAN)
}

func init() {
	if os.Getenv("DEBUG") != "" {
		Level = 0
	}
	fi, _ := os.Stderr.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// the stderr is a pipe
		IsTerminal = false
	}
	if os.Getenv("COLOR_TERMINAL") != "" {
		IsTerminal = true
	}
}

func D(i int) Logger {
	return std.D(i - 1)
}

func (l *Logger) D(i int) Logger {
	return Logger{l.depth + i, NORMAL, l.reqid, l.Logger}
}

func (l *Logger) Pretty(os ...interface{}) {
	PRETTY := "[PRETTY] "
	content := ""
	colors := []string{"31", "32", "33", "35"}
	for i, o := range os {
		if ret, err := json.MarshalIndent(o, "", "\t"); err == nil {
			content += color(colors[i%len(colors)], string(ret)) + "\n"
		}
	}
	l.Output(2, PRETTY+content)
}

func (l *Logger) Print(o ...interface{}) {
	l.Output(2, sprint(o))
}
func (l *Logger) Printf(layout string, o ...interface{}) {
	l.Output(2, sprintf(layout, o))
}
func (l *Logger) Println(o ...interface{}) {
	l.Output(2, sprint(o))
}

func (l *Logger) Debug(o ...interface{}) {
	if Level > DEBUG {
		return
	}

	DEBUG := "[D] "
	l.Output(2, DEBUG+sprint(o))
}
func (l *Logger) Debugf(f string, o ...interface{}) {
	if Level > DEBUG {
		return
	}

	DEBUG := "[D] "
	l.Output(2, DEBUG+sprintf(f, o))
}

func (l *Logger) Info(o ...interface{}) {
	if Level > INFO {
		return
	}

	INFO := "[" + Green("I") + "] "
	l.Output(2, INFO+sprint(o))
}
func (l *Logger) Infof(f string, o ...interface{}) {
	if Level > INFO {
		return
	}

	INFO := "[" + Green("I") + "] "
	l.Output(2, INFO+sprintf(f, o))
}

func (l *Logger) Warn(o ...interface{}) {
	if Level > WARN {
		return
	}

	WARN := "[" + Pink("W") + "] "
	l.Output(2, WARN+sprint(o))
}
func (l *Logger) Warnf(f string, o ...interface{}) {
	if Level > WARN {
		return
	}

	WARN := "[" + Pink("W") + "] "
	l.Output(2, WARN+sprintf(f, o))
}

func (l *Logger) Error(o ...interface{}) {
	if Level > ERROR {
		return
	}

	ERROR := "[" + Red("E") + "] "
	l.Output(2, ERROR+sprint(o))
}
func (l *Logger) Errorf(f string, o ...interface{}) {
	if Level > ERROR {
		return
	}

	ERROR := "[" + Red("E") + "] "
	l.Output(2, ERROR+sprintf(f, o))
}

func (l *Logger) Panic(o ...interface{}) {
	if Level > PANIC {
		return
	}

	PANIC := "[" + Red("PANIC") + "] "
	l.Output(2, PANIC+sprint(o))
	panic(o)
}
func (l *Logger) Panicf(f string, o ...interface{}) {
	if Level > PANIC {
		return
	}

	PANIC := "[" + Red("PANIC") + "] "
	info := sprintf(f, o)
	l.Output(2, PANIC+info)
	panic(info)
}

func (l *Logger) Fatal(o ...interface{}) {
	if Level > FATAL {
		return
	}

	FATAL := "[" + Red("F") + "] "
	l.Output(2, FATAL+sprint(o))
	os.Exit(1)
}
func (l *Logger) Fatalf(f string, o ...interface{}) {
	if Level > FATAL {
		return
	}

	FATAL := "[" + Red("F") + "] "
	l.Output(2, FATAL+sprintf(f, o))
	os.Exit(1)
}

func (l *Logger) SetReqId(reqid string) (oldReqId string) {
	oldReqId = l.reqid
	l.reqid = reqid
	return
}

func (l *Logger) SetOutput(w io.Writer) {
	l.Logger.SetOutput(w)
	IsTerminal = false
	if os.Getenv("COLOR_TERMINAL") != "" {
		IsTerminal = true
	}
}

func (l *Logger) Struct(o ...interface{}) {
	STRUCT := "[STRUCT] "
	items := make([]interface{}, 0, len(o)*2)
	for _, item := range o {
		items = append(items, item, item)
	}
	layout := strings.Repeat(", %T(%+v)", len(o))
	if len(layout) > 0 {
		layout = layout[2:]
	}
	l.Output(2, STRUCT+sprintf(layout, items))
}

func (l *Logger) PrintStack() {
	Info(string(l.Stack()))
}

func (l *Logger) Stack() []byte {
	a := make([]byte, 1024*1024)
	n := runtime.Stack(a, true)
	return a[:n]
}

func (l *Logger) Output(calldepth int, s string) error {
	calldepth += l.depth + 1
	if l.Logger == nil {
		l.Logger = stdGoLog
	}
	return l.Logger.Output(calldepth, l.makePrefix(calldepth)+s)
}

func (l *Logger) Todo(o ...interface{}) {
	TODO := "[" + color("35", "TODO") + "] "
	l.Output(2, TODO+sprint(o))
}

func (l *Logger) makePrefix(calldepth int) string {
	tags := make([]string, 0, 3)
	if l.verbose > MORE {
		pc, f, line, _ := runtime.Caller(calldepth)
		name := runtime.FuncForPC(pc).Name()
		name = path.Base(name) // only use package.funcname
		f = path.Base(f)       // only use filename

		pos := name + "@" + f + ":" + strconv.Itoa(line)
		tags = append(tags, pos)
	}
	if l.reqid != "" {
		tags = append(tags, l.reqid)
	}
	if len(tags) == 0 {
		return ": "
	}
	return "[" + strings.Join(tags, "][") + "]: "
}

func sprint(o []interface{}) string {
	decodeTrackError(o)
	return joinInterface(o, " ")
}

func sprintf(f string, o []interface{}) string {
	decodeTrackError(o)
	return fmt.Sprintf(f, o...)
}

func decodeTrackError(o []interface{}) {
	for idx, obj := range o {
		if te, ok := obj.(*TrackError); ok {
			o[idx] = te.StackError()
		}
	}
}

type RotateWriter struct {
	lock     sync.Mutex
	filename string // should be set to the actual filename
	fp       *os.File
	fTime    time.Time
}

// Make a new RotateWriter. Return nil if error occurs during setup.
func NewRotateWriter(filename string) *RotateWriter {
	w := &RotateWriter{filename: filename}
	err := w.Rotate()
	if err != nil {
		//fmt.Println(err)
		return nil
	}
	go func() {
		for {
			time.Sleep(10 * time.Second)
			if w.fp != nil {
				w.fp.Sync()
			}
		}
	}()
	return w
}

// Write satisfies the io.Writer interface.
func (w *RotateWriter) Write(output []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.fTime.Day() != time.Now().Day() {
		if err := w.Rotate(); err != nil {
			Println(err)
			return 0, err
		}
	}
	return w.fp.Write(output)
}

// Perform the actual act of rotating and reopening file.
func (w *RotateWriter) Rotate() (err error) {

	// Close existing file if open
	if w.fp != nil {
		err = w.fp.Close()
		w.fp = nil
		if err != nil {
			return
		}
		// Rename dest file if it already exists
		_, err = os.Stat(w.filename)
		if err == nil {
			err = os.Rename(w.filename, w.filename+"."+time.Now().Format("2006-01-02"))
			if err != nil {
				Println(err)
			}
		}
	}
	// Create a file.
	w.fp, err = os.OpenFile(w.filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Println(err)
	}
	w.fTime = time.Now()
	return
}

func (w *RotateWriter) Close() {
	if w.fp != nil {
		w.fp.Close()
		w.fp = nil
	}
}
