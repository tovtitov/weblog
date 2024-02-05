package weblog

/*			TODO:
Uid from string to uuid, but latency remained as string
определиться при вствке в базу
*/

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	TAB                   = "\t"
	NEW_LINE              = "\n"
	PATH_SEPARATOR string = string(os.PathSeparator)

	// this has to be ordered by logging data size
	// https://stackoverflow.com/questions/2031163/when-to-use-the-different-log-levels
	LOG_ERROR = 1
	LOG_INFO  = 2
	LOG_TRACE = 3
	LOG_DEBUG = 4

	// MaxUint      = ^uint(0)
	// MinUint      = 0
	// MaxInt       = int(MaxUint >> 1)
	// MinInt       = -MaxInt - 1
	MAX_INT_1KB  = 1024
	MAX_INT_10KB = 10240
	// MaxInt_100KB = 102400
	// MaxInt_1MB   = 1048576
	// MaxInt_5MB   = 5242880
	// MaxInt_10MB  = 10485760

	SLEEP_PING_PERIOD_SEC = 3

	LBL_ERROR = "ERROR"

	TIME_LAYOUT = "2006-01-02 15:04:05.9999999999"

	FieldCmdLen          = 75
	FieldMaxRequestQSLen = 1024
	FieldMaxRequestLen   = 10485760

	// FieldMaxTextLen      = -1
	FieldMaxContentType = 75 // rqsr, rsqt
	FieldMaxIPLength    = 250
)

var (
	_log_mark          string // to reduce stacktrace path
	_loglevel          int    = LOG_DEBUG
	_logFileCountLimit int    = -1
	_logPath           string
	_fileNameFormat    []string  = []string{"D", "T"} // default file format
	_uuidInstanceID    uuid.UUID = uuid.New()         //[16]byte
	_uuidDefault       uuid.UUID                      //[16]byte

	_log_format = `^^^	datetime	err	cmd	code	latency	ip	srvc	rqct	rsct	reqid	uid	rqqs	useragent
rq
rs
`
	_fieldsDefs              = []string{"datetime", "err", "cmd", "code", "latency", "ip", "srvc", "rqct", "rsct", "reqid", "uid", "rqqs", "useragent", "rq", "rs"}
	_recordDelimmiter string = "^^^"

	_muLogWrite = &sync.Mutex{}
	_fileLog    *os.File

	// parsed log file default header
	_rowtags       []string
	_tagDelimiters []string

	_rxTag        = regexp.MustCompile("^[a-zA-Z]{1,25}$")
	_rxLines      = regexp.MustCompile(NEW_LINE)
	_rxSpaces     = regexp.MustCompile(`\s`)
	_rxCmd        = regexp.MustCompile(`^([\d_a-zA-Z]{1,25}\.)?([\d_a-zA-Z]{1,25})\.([\d_a-zA-Z]{1,25})$`)
	_rxSrvAbbr    = regexp.MustCompile(`^[\d_A-Z]{5}$`)
	_rxFileFormat = regexp.MustCompile(`^[ADTU]{1,4}$`)
	_rxUrl        = regexp.MustCompile(`^https?:\/\/(?:www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}$`)
	// _rxIP         = regexp.MustCompile(`^https?:\/\/((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}$`)
	_rxIPv4v6        = regexp.MustCompile(`((^\s*((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))\s*$)|(^\s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))(%.+)?\s*$))`)
	_rxIPv4v6_simple = regexp.MustCompile(`^[0-9.:A-Fa-f, \[\]]{1,150}$`)
	_rxUserAgent     = regexp.MustCompile(`[^\t \.,_;\(\)\/a-zA-Z\d]`)

	// https://www.regextester.com/104038
	// rxIp     = regexp.MustCompile(`((^\s*((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))\s*$)|(^\s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))(%.+)?\s*$))`)

	//
	_infoMessageBuf  *bytes.Buffer
	_errorMessageBuf *bytes.Buffer
	// _period               int      = 111 //111 - day, 11 - hour, 1 - minute
	// _periodCount          int      = 1
	_isWebLogSrvAvailable = false
	_isAppLive            = true
	_isInitialized        = false
	_isStandalone         = true // log file run on WebLogSrv

	_mapHeaders  map[string]*logFormatInfo //parsed log file header formats
	_serviceAbbr string

	client                *http.Client
	SERVER_URL            string = "http://localhost"
	SERVER_URL_LOG_RECORD        = ":5678/record.write?servicename="
	SERVER_URL_LOG_BATCH         = ":5678/batch.write?servicename="
	QSKEY_SRV_NAME               = "servicename"
)

// for request/response only. Use weblog funcs for start/stop logging
type Logger struct {
	time          *time.Time
	timeStr       string
	reqid         uuid.UUID
	uid           uuid.UUID
	user          interface{}
	code          int
	codeStr       string
	cmd           string
	latency       int64
	latencyStr    string
	ip            string
	rqct          string
	rsct          string
	rqqs          string
	useragent     string
	useragent_cmd string
	// rq                 string
	is_response_binary bool // for images
	is_request_binary  bool // for images

	Enabled     bool // true by default. Set false not to log
	serviceName string
	loglevel    int

	// logRawRecord     string
	responseBuffer   *bytes.Buffer
	requestBuffer    *bytes.Buffer
	stacktraceBuffer *bytes.Buffer
	recRawBuffer     *bytes.Buffer
	mapQS            url.Values

	// w.CookiesRequest = append(w.CookiesRequest, &http.Cookie{})
	CookiesRequest []*http.Cookie
	// w.CookiesResponse = append(w.CookiesResponse, &http.Cookie{})
	CookiesResponse []*http.Cookie
}

// START | STOP

// initializes appliaction log from weblog.config file, located next to app file, or with default settings.
func Init() { _initialize("") }

// initializes with path where weblog.config to be found.
// f.e. app starts within docker and app calls this func to define host located weblog.config file.
// and here you can set it from app.
func InitFromPath(configPath string) { _initialize(configPath) }

func _initialize(configPath string) {

	defer func() {
		r := recover()
		if r != nil {
			_recover("can not create logger instance: ", true)
		}
	}()

	var (
		err error
	)

	err = initialize_config(configPath)
	if err != nil {
		fmt.Printf("can not parse log config file: %s\n", err.Error())
		os.Exit(1)
	}

	_infoMessageBuf = &bytes.Buffer{}
	_errorMessageBuf = &bytes.Buffer{}

	if len(_logPath) == 0 {
		_logPath, err = getDefaultLogPath()
		if err != nil {
			printError("log folder not defined")
			os.Exit(1)
		}
	}
	fmt.Println("default log path: [" + _logPath + "]")

	_uuidInstanceID = uuid.New()

	if !_isStandalone {
		go initializeClient()
	}

	// validate log file accessability

	// ************************************************
	// !!!!creates log file within docker if it runs there!!!
	// use SetLogPath() to change log path after initialization
	// ************************************************
	var fileExisted = false
	_fileLog, fileExisted, err = createLogFile(_logPath)
	if err != nil {
		fmt.Printf("can not create log file: %s\n", err.Error())
		os.Exit(1)
	}

	// no need in log file if log service available
	if _isWebLogSrvAvailable && !fileExisted {
		_fileLog.Close()
		os.Remove(_logPath)
		_fileLog = nil
	}

	_isInitialized = true

}

func NewLogger() *Logger {

	w := &Logger{}
	w.Enabled = true
	w.requestBuffer = &bytes.Buffer{}
	w.responseBuffer = &bytes.Buffer{}
	w.recRawBuffer = &bytes.Buffer{}
	w.requestBuffer.Grow(MAX_INT_10KB)
	w.responseBuffer.Grow(MAX_INT_10KB)
	w.stacktraceBuffer = &bytes.Buffer{}
	w.stacktraceBuffer.Grow(MAX_INT_10KB)
	w.SetTime(time.Now())
	w.SetResponseCode(200)
	w.rqct = "text/plain"
	// w.logRawRecord = ""
	w.loglevel = _loglevel
	w.is_request_binary = false
	w.is_response_binary = false

	return w
}

func LogPath() string { return _logPath }

// weblogsrv url
func LogServerURL() string { return SERVER_URL }

func InstanceID() uuid.UUID { return _uuidInstanceID }

func IsServiceAbbreviation(srvabbr string) bool { return _rxSrvAbbr.MatchString(srvabbr) }

func GetLogFilePath() string {
	if _fileLog == nil {
		return ""
	}
	return _fileLog.Name()
}

// change log directory, and create new log file in it (if it does not exist).
// if not set, directory "logs" will be created next to the executable.
func SetLogPath(logdir string) error {

	if len(logdir) == 0 {
		return errors.New("log path expected")
	}

	logdir = strings.TrimSpace(logdir)
	if len(logdir) == 0 {
		return errors.New("log path expected")
	}
	if logdir == _logPath {
		return nil
	}

	if !strings.HasSuffix(logdir, PATH_SEPARATOR) {
		logdir = logdir + PATH_SEPARATOR
	}
	err := validatePath(logdir, true)
	if err != nil {
		return err
	}
	_logPath = logdir

	fileLog, _, err := createLogFile(logdir)
	if err != nil {
		fmt.Println("can not cfeate log file in new directory: " + logdir + "\nERROR:" + err.Error())
		AddError(fmt.Sprintf("can not cfeate log file in new directory: %s\n%s", logdir, err.Error()))
		WriteTask()
		return err
	}

	_muLogWrite.Lock()
	Close()
	_fileLog = fileLog
	_muLogWrite.Unlock()

	return nil
}

// May be used in log file names, when many instances write logs into one folder.
// Every instance writes log into its own file to avoid concurrency problem.
// To use it, add U in SetFileNameFormat() parameter.
// If it is not set, it is generated automatically on initialization.
func SetInstanceID(srvid uuid.UUID) {
	_uuidInstanceID = srvid
}

// on application start / stop
func AddInfo(val string) {
	fmt.Println(val)
	sz := len(val)
	if sz == 0 {
		return
	}
	buf := _infoMessageBuf
	diff := buf.Cap() - buf.Len()
	if len(val) > diff {
		buf.Grow(sz + buf.Len() + 10)
	}
	buf.WriteString(val)
	buf.WriteString(NEW_LINE)
}

// on application start / stop
func AddError(val string) {

	sz := len(val)
	if sz == 0 {
		return
	}
	buf := _errorMessageBuf
	diff := buf.Cap() - buf.Len()
	if len(val) > diff {
		buf.Grow(sz + buf.Len() + 10)
	}
	buf.WriteString(val)
	buf.WriteString(NEW_LINE)
}

// on application start / stop
var _latency int64

// latency - service start time
func WriteStart(latency int64) *Logger {

	if !_isStandalone {
		for i := 0; i < 10; i++ {
			if !_isWebLogSrvAvailable {
				time.Sleep(300 * time.Millisecond)
			} else {
				break
			}
		}
	}

	_latency = latency
	return write(1)
}

// latency - service total life time
func WriteStop(latency int64) *Logger {
	_latency = latency
	return write(2)
}
func WriteTask() *Logger {
	return write(3)
}

// AT STARTUP, recovery after a panic, with exiting the application
func PanicOnStart(errmsg string) {
	panic(errmsg, true)
}
func PanicOnStop(errmsg string) {
	panic(errmsg, false)
}

// recovery after a panic during the start of the application
// with exiting the application
// usage: weblog.Recover(recover())
func RecoverOnStart(r interface{}) {
	_recover(r, true)
}
func RecoverOnStop(r interface{}) {
	_recover(r, false)
}
func _recover(r interface{}, on_start bool) {

	if r == nil {
		return
	}
	if on_start {
		switch r.(type) {
		case string:
			PanicOnStart(r.(string))
		case error:
			PanicOnStart(r.(error).Error())
		default:
			PanicOnStart("unknown panic")
		}
	} else {
		switch r.(type) {
		case string:
			PanicOnStop(r.(string))
		case error:
			PanicOnStop(r.(error).Error())
		default:
			PanicOnStop("unknown panic")
		}
	}

}

// closes locacl log file only. If not standalone, call extended CloseServerLog() method:
// it closes local log file and stops remote client cycle by setting flag _isAppLive to false
func Close() {

	var err error
	defer func() {
		if r := recover(); r != nil {
			fmt.Print("PANIC: on Close: ")
			fmt.Println(r)
		}
		if err != nil {
			fmt.Println("ERROR: on Close: " + err.Error())
			// printError(err.Error())
			// os.WriteFile(_logPath+"logcrash.txt", []byte(err.Error()), 0777)
		}
	}()
	if _fileLog != nil {
		err = _fileLog.Sync()
		if err != nil {
			return
		}
		err = _fileLog.Close()
		if err != nil {
			return
		}
		err = archihveFile()
		if err != nil {
			return
		}
		_fileLog = nil
	}
}

// sets _isAppLive = false and breaks
func CloseServerLog() {
	if _fileLog != nil {
		_fileLog.Sync()
		_fileLog.Close()
		archihveFile()
		_fileLog = nil
	}
	_isAppLive = false
	time.Sleep(SLEEP_PING_PERIOD_SEC * time.Second) // wait background cycle exit
}

// NormalizeNewlines normalizes \r\n (windows) and \r (mac) into \n (unix)
func NormalizeNewlines(val string) string {
	// https://www.programming-books.io/essential/go/normalize-newlines-1d3abcf6f17c4186bb9617fa14074e48

	if len(val) == 0 {
		return ""
	}
	d := []byte(val)
	// replace CR LF \r\n (windows) with LF \n (unix)
	d = bytes.Replace(d, []byte{13, 10}, []byte{10}, -1)
	// replace CF \r (mac) with LF \n (unix)
	d = bytes.Replace(d, []byte{13}, []byte{10}, -1)

	return string(d)
}
