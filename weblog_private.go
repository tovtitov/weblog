package weblog

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PRIVATE

var (
	_lastDay   int
	_lastMonth int
)

type logFormatInfo struct {
	Columns          []string
	ColumnDelimiters []string
	RecordDelimmiter string
}

func (info *logFormatInfo) validate() error {
	if !(info.Columns != nil && len(info.Columns) > 0) {
		return errors.New("no columns found")
	}
	if !(info.ColumnDelimiters != nil && len(info.ColumnDelimiters) > 0) {
		return errors.New("no column delimiters found")
	}
	if len(info.RecordDelimmiter) == 0 {
		return errors.New("no record delimiter found")
	}
	// ^^^ datetime err cmd code latency ip srvc rqct rsct reqid uid rqqs\r\nrq\r\nrs
	errFields := ""
	for _, r := range info.Columns {
		if !slices.Contains(_fieldsDefs, r) {
			errFields = errFields + r + ","
		}
	}
	if len(errFields) > 0 {
		errFields = errFields[:len(errFields)-1]
		return errors.New("unsupported fields: " + errFields)
	}
	return nil
}

func logTxt(msg *string) {

	if !_isInitialized {
		fmt.Println("log is not initialized!")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Print("PANIC: on logTxt: ")
			fmt.Println(r)
		}
	}()

	// debug
	// fmt.Println(*msg)

	idx := 1 // 10 times tries log to WebLogSrv or file
NEXT:

	// if _serviceAbbr == "LOGER" {
	// 	sql.DB..LoadRecordToHeap(oLogRec, wrapper)
	// }
	if !_isStandalone && _isWebLogSrvAvailable {
		*msg = _log_format + *msg
		fmt.Println(*msg)
		arr := []byte(*msg)
		_, code, err := sendMessage(SERVER_URL+SERVER_URL_LOG_RECORD+_serviceAbbr, &arr)
		if code == 200 && err == nil {
			// message was sent to server successfully
			if _fileLog != nil {
				// no need in log file, drops logs on server
				_muLogWrite.Lock()
				Close()
				_muLogWrite.Unlock()
				// go sendBatch(_fileLog.Name())

			}
			return
		}
	}

	if _fileLog == nil {

		err := createLogFileAgain()
		if err != nil {

			// no service, no file/ Try 10 times before exit.
			time.Sleep(100 * time.Millisecond)
			if idx > 10 {
				fmt.Println(msg)
				return
			}
			idx++
			goto NEXT
		}
	}

	currTime := time.Now()

	// if _period == 111 {
	// 	if currTime.Day() != _lastDay {
	// 		createLogFileAgain()
	// 	}
	// } else if _period == 11 {
	// 	if currTime.Hour() != _lastHour {
	// 		createLogFileAgain()
	// 	}
	// } else { // _period == 1
	// 	if currTime.Minute() != _lastMinute {
	// 		createLogFileAgain()
	// 	}
	// }

	if currTime.Day() != _lastDay /*|| TestDateFlag*/ {
		// if currTime.Day() != _lastDay || currTime.Month() != time.Month(_lastMonth) /*|| TestDateFlag*/ {
		// if currTime.Minute() != _lastMinute /*|| TestDateFlag*/ {
		createLogFileAgain()
	}

	// file may be closed by other thread (service became available, file was nilled)
	if _fileLog != nil {
		_muLogWrite.Lock()
		_fileLog.WriteString(*msg)
		fmt.Println(*msg)
		_muLogWrite.Unlock()
		if _fileLog == nil {
			time.Sleep(100 * time.Millisecond)
			if idx > 10 {
				fmt.Println(msg) // no service, no file
				return
			}
			idx++
			goto NEXT
		}
	} else {
		// no service, no file/ Try 10 times before exit.
		time.Sleep(100 * time.Millisecond)
		if idx > 10 {
			fmt.Println(msg) // no service, no file
			return
		}
		idx++
		goto NEXT
	}
}

func printStackTrace(sb *bytes.Buffer) {

	var (
		funcName string
		skip     int
	)

	skip = 0
	hasLogMark := len(_log_mark) > 0
	for {

		if pc, file, line, ok := runtime.Caller(skip); ok {

			skip += 1

			fmt.Println(file)

			fn := runtime.FuncForPC(pc)
			if fn != nil {
				funcName = fn.Name()
				if hasLogMark && !strings.Contains(funcName, _log_mark) {
					skip += 1
					continue
				}
			} else {
				funcName = "unknown method"
			}
			// sl := fmt.Sprintf("%d", line)

			sb.WriteString(funcName)
			sb.WriteString("\t")
			sb.WriteString(fmt.Sprint(line))
			sb.WriteString("\n")

		} else {
			break
		}
		skip += 1

	}
	sb.WriteString("\n")
}

func createLogFileAgain() error {

	defer func() {
		if r := recover(); r != nil {
			fmt.Print("PANIC: on createLogFileAgain: ")
			fmt.Println(r)
		}
	}()

	_muLogWrite.Lock()
	Close()
	_muLogWrite.Unlock()

	var (
		strErr string
		err    error
	)
	_fileLog, _, err = createLogFile(_logPath)
	if err != nil {
		strErr = fmt.Sprintf("can not cfeate log file: %s\n%s", _logPath, err.Error())
		printError(strErr)
		AddError(strErr)
		WriteTask()
		return printError(strErr)
	}

	return nil
}

func find(root, ext string) []string {
	var a []string
	filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ext {
			a = append(a, s)
		}
		return nil
	})
	return a
}

type file_item struct {
	Path     string
	Modified time.Time
}

func removeOldLogFiles(logpath string) {

	var fitems []*file_item
	//collect
	for _, src := range find(logpath, ".log") {

		stat, err := os.Stat(src)
		if err != nil {
			continue
		}

		// delete empty files
		diff := int(stat.Size()) - len(_log_format)
		if -10 < diff && diff < 10 {
			os.Remove(src)
		} else {
			fitem := &file_item{}
			fitem.Path = src
			fitem.Modified = stat.ModTime()
			fitems = append(fitems, fitem)
		}
	}

	// _logFileCountLimit == -1 - DO NOTHING
	if _logFileCountLimit > 0 && len(fitems) > 0 && len(fitems) >= _logFileCountLimit {

		sort.Slice(fitems, func(i, j int) bool {
			return fitems[i].Modified.After(fitems[j].Modified)
		})
		for i := _logFileCountLimit; i < len(fitems); i++ {
			os.Remove(fitems[i].Path)
		}

	}
}

// creates new log file
// params: log directory
// returns:
// - file reference,
// - flag, if the file has already been,
// - error
func createLogFile(logpath string) (*os.File, bool, error) {

	defer func() {
		if r := recover(); r != nil {
			fmt.Print("PANIC: on createLogFile: ")
			fmt.Println(r)
		}
	}()

	removeOldLogFiles(logpath)

	var (
		err              error
		fileLog          *os.File
		fileExistsBefore bool
	)
	if len(logpath) == 0 {
		return nil, false, nil
	}

	currTime := time.Now()

	sb := &bytes.Buffer{}
	sb.Grow(1000)
	joinStringBuffP(sb, logpath)
	for _, ch := range _fileNameFormat {
		switch ch {
		case "A":
			joinStringBuffP(sb, _serviceAbbr, ".")
		case "D":
			joinStringBuffP(sb, currTime.Format("2006-01-02"), ".")
		case "T":
			joinStringBuffP(sb, currTime.Format("15-04-05"), ".")
		case "U":
			joinStringBuffP(sb, _uuidInstanceID.String(), ".")
		}
	}

	joinStringBuffP(sb, "log")
	logFilePath := sb.String()
	fmt.Println("logFilePath: " + logFilePath)
	sb.Reset()

	if _, err := os.Stat(logFilePath); errors.Is(err, os.ErrNotExist) {
		fileExistsBefore = false
	} else {
		fileExistsBefore = true
	}
	// fileLog, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_RDWR|os.O_CREATE, os.ModeExclusive)
	fileLog, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		e := "can not open log file: " + err.Error()
		fmt.Println(e)
		return nil, fileExistsBefore, errors.New(e)
	}
	_lastDay = currTime.Day()
	_lastMonth = int(currTime.Month())
	// _lastHour = int(currTime.Hour())
	// _lastMinute = currTime.Minute()

	if !fileExistsBefore {
		_, err = fileLog.WriteString(_log_format) // log file write check
		if err != nil {
			fmt.Println("log file write check error: " + err.Error())
		}
	}

	return fileLog, fileExistsBefore, err
}

func printError(errmsg string) error {
	err := errors.New(errmsg)
	fmt.Println(errmsg)
	return err
}

func joinStringBuffP(sb *bytes.Buffer, elem ...string) {

	for _, e := range elem {
		if len(e) > 0 {
			sb.WriteString(e)
		}
	}
}

// // extract to client because config, or internal test strings may be used
func getDefaultLogPath() (string, error) {

	var (
		err error
		// logPathDefault string
	)
	if len(_logPath) == 0 {
		_logPath, err = os.Executable()
		if err != nil {
			fmt.Printf("can not get current path:  %s\n", err.Error())
			return "", err
		}
	}
	dir := filepath.Dir(_logPath)
	if !strings.HasSuffix(_logPath, "/logs/") {
		if strings.EqualFold(dir, "/") {
			_logPath = dir + "logs" + string(os.PathSeparator)
		} else {
			_logPath = dir + string(os.PathSeparator) + "logs" + string(os.PathSeparator)
		}
	}

	err = os.MkdirAll(_logPath, os.ModePerm)
	if err != nil {
		return "", printError(fmt.Sprintf("can not create folder: %s\n%s", _logPath, err.Error()))
	}

	return _logPath, nil
}

func write(mode int) *Logger {

	sb := &bytes.Buffer{}
	strInfo := _infoMessageBuf.String()
	intInfo := len(strInfo)

	strErr := _errorMessageBuf.String()
	intErr := len(strErr)

	intLen := intInfo + intErr
	sb.Grow(intLen)

	logger := NewLogger()

	sb.WriteString(_recordDelimmiter)
	sb.WriteString(_tagDelimiters[0])
	// `datetime err cmd code latency ip srvc rqct rsct reqid uid rqqs
	// rq
	// rs`
	for i, tab := range _rowtags {
		switch tab {
		case "datetime":
			logger.SetTime(time.Now())
			sb.WriteString(logger.timeStr)
			sb.WriteString(_tagDelimiters[i])
		case "err":
			if intErr > 0 {
				sb.WriteString(LBL_ERROR)
				logger.AddResponse(LBL_ERROR)
			}
			sb.WriteString(_tagDelimiters[i])
		case "code":
			if intErr == 0 {
				sb.WriteString(strconv.Itoa(mode))
				logger.SetResponseCode(mode)
			} else {
				sb.WriteString("500")
				logger.SetResponseCode(500)
			}
			sb.WriteString(_tagDelimiters[i])
		case "srvc":
			sb.WriteString(_serviceAbbr)
			logger.SetServiceName(_serviceAbbr)
			sb.WriteString(_tagDelimiters[i])
		case "cmd":
			switch mode {
			case 1:
				sb.WriteString("service.start")
				logger.SetCommand("service.start")
			case 2:
				sb.WriteString("service.stop")
				logger.SetCommand("service.stop")
			default:
			}
			sb.WriteString(_tagDelimiters[i])
		case "latency":
			sb.WriteString(strconv.FormatInt(_latency, 10))
			sb.WriteString(_tagDelimiters[i])
		case "ip", "rqct", "rsct", "reqid", "uid", "rqqs":
			sb.WriteString(_tagDelimiters[i])
		case "rq":
			// continue
			sb.WriteString("rq:")
			sb.WriteString(_tagDelimiters[i])
		case "rs":
			sb.WriteString("rs:")
			if intInfo > 0 {
				sb.WriteString(strInfo)
				logger.AddResponse(strInfo)
			}
			if intErr > 0 {
				sb.WriteString(strErr)
				logger.AddResponse(strErr)
			}
			sb.WriteString(_tagDelimiters[i])
		}
	}
	res := sb.String()
	logTxt(&res)
	sb.Reset()
	_infoMessageBuf.Reset()
	_errorMessageBuf.Reset()
	return logger
}

func recoverTaskAndWork(r interface{}) {

	if r == nil {
		return
	}
	switch r.(type) {
	case string:
		panicTaskAndWork(r.(string))
	case error:
		panicTaskAndWork(r.(error).Error())
	default:
		panicTaskAndWork("unknown panic")
	}
}

func panicTaskAndWork(errmsg string) {

	if len(errmsg) > 0 {
		_errorMessageBuf.WriteString(errmsg)
		_errorMessageBuf.WriteString("\n")
	}

	printStackTrace(_errorMessageBuf)

	WriteTask()

}

func panic(errmsg string, is_start bool) {

	if len(errmsg) > 0 {
		_errorMessageBuf.WriteString(errmsg)
		_errorMessageBuf.WriteString("\n")
		printStackTrace(_errorMessageBuf)
		if is_start {
			WriteStart(0)
		} else {
			WriteStop(0)
		}
	}
	Close()
	os.Exit(1)
}

// for convenience purpose (compact writing)
func joinString(elem ...string) string {

	lenArgs := 0
	if len(elem) == 0 {
		return ""
	}
	for _, str := range elem {
		lenArgs = lenArgs + len(str)
	}

	var sb bytes.Buffer
	sb.Grow(lenArgs)
	for _, e := range elem {
		if e != "" {
			sb.WriteString(e)
		}
	}
	res := sb.String()
	sb.Reset()
	return res
}

func archihveFile() error {

	fi, err := os.Stat(_fileLog.Name())
	if err != nil {
		return err
	}
	diff := int(fi.Size()) - len(_log_format)
	if -10 < diff && diff < 10 {
		os.Remove(_fileLog.Name())
		return err
	}

	fdir, fname := filepath.Split(_fileLog.Name())
	err = os.Rename(_fileLog.Name(), fdir+"_"+fname)
	if err != nil {
		return err
		// AddError(err.Error())
		// WriteTask()
	}
	return nil
}

func LogTxtTest(msg *string) {

	if !_isInitialized {
		fmt.Println("log is not initialized!")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Print("PANIC: logTxt: ")
			fmt.Println(r)
		}
	}()

	// debug
	// fmt.Println(*msg)

	idx := 1 // 10 times tries log to WebLogSrv or file
NEXT:

	// if _serviceAbbr == "LOGER" {
	// 	sql.DB..LoadRecordToHeap(oLogRec, wrapper)
	// }
	if !_isStandalone && _isWebLogSrvAvailable {
		*msg = _log_format + *msg
		fmt.Println(*msg)
		arr := []byte(*msg)
		_, code, err := sendMessage(SERVER_URL+SERVER_URL_LOG_RECORD+_serviceAbbr, &arr)
		if code == 200 && err == nil {
			// message was sent to server successfully
			if _fileLog != nil {
				// no need in log file, drops logs on server
				_muLogWrite.Lock()
				Close()
				_muLogWrite.Unlock()
				// go sendBatch(_fileLog.Name())

			}
			return
		}
	}

	if _fileLog == nil {
		err := createLogFileAgain()
		if err != nil {
			// no service, no file/ Try 10 times before exit.
			time.Sleep(100 * time.Millisecond)
			if idx > 10 {
				fmt.Println(msg)
				return
			}
			idx++
			goto NEXT
		}
	}

	currTime := time.Now()
	if currTime.Day() != _lastDay || currTime.Month() != time.Month(_lastMonth) /*|| TestDateFlag*/ {
		createLogFileAgain()
	}

	// file may be closed by other thread (service became available, file was nilled)
	if _fileLog != nil {
		_muLogWrite.Lock()
		if _fileLog != nil {
			_fileLog.WriteString(*msg)
			fmt.Println(*msg)
		}
		_muLogWrite.Unlock()
		if _fileLog == nil {
			time.Sleep(100 * time.Millisecond)
			if idx > 10 {
				fmt.Println(msg) // no service, no file
				return
			}
			idx++
			goto NEXT
		}
	} else {
		// no service, no file/ Try 10 times before exit.
		time.Sleep(100 * time.Millisecond)
		if idx > 10 {
			fmt.Println(msg) // no service, no file
			return
		}
		idx++
		goto NEXT
	}
}

// func deleteTestLogFile() {
// 	fp := GetLogFilePath()
// 	if len(fp) == 0 {
// 		return
// 	}
// 	var (
// 		fi  fs.FileInfo
// 		err error
// 	)
// 	if fi, err = os.Stat(fp); errors.Is(err, os.ErrNotExist) {
// 		return
// 	}
// 	if fi.Size() > 1024 {
// 		return
// 	}
// 	barr, err := os.ReadFile(fp)
// 	if err != nil {
// 		return
// 	}
// 	pos := strings.Index(string(barr), "\n\n")

// 	diff := int(fi.Size()) - pos
// 	if -10 < diff && diff < 10 {
// 		os.Remove(fp)
// 	}

// }
