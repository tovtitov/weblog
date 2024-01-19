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
	TAB      = "\t"
	NEW_LINE = "\n"

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

	FieldMaxIPLength = 250
)

var (
	_log_mark       string // to reduce stacktrace path
	_path_separator string = string(os.PathSeparator)
	_log_format            = `^^^	datetime	err	cmd	code	latency	ip	srvc	rqct	rsct	reqid	uid	rqqs	useragent
rq
rs
`
	_fieldsDefs = []string{"datetime", "err", "cmd", "code", "latency", "ip", "srvc", "rqct", "rsct", "reqid", "uid", "rqqs", "useragent", "rq", "rs"}

	_recordDelimmiter string = "^^^"

	_loglevel                        int = LOG_TRACE
	_logFileCountLimitMin            int = 10
	_logFileCountLimit               int = 100
	_muLogWrite                          = &sync.Mutex{}
	_fileLog                         *os.File
	_lastDay, _lastHour, _lastMinute int = 0, 0, 0 //  for test only: _lastDay, _lastHour, _lastMinute int = 0, 0, 0
	_logPath                         string
	_uuidDefault                     uuid.UUID //[16]byte
	_uuidInstanceID                  uuid.UUID //[16]byte

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
	_infoMessageBuf       *bytes.Buffer
	_errorMessageBuf      *bytes.Buffer
	_fileNameFormat       []string = []string{}
	_period               int      = 111 //111 - day, 11 - hour, 1 - minute
	_periodCount          int      = 1
	_isWebLogSrvAvailable          = false
	_isAppLive                     = true
	_isInitialized                 = false
	_isStandalone                  = true // log file run on WebLogSrv

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

// initializes just single appliaction log
func Init() {

	_initialize("", true, "")

}

// - logHeaderFormat - example: "^^^\tdatetime\terr\tcmd\tcode\tlatency\tip\tsrvc\trqct\trsct\treqid\tuid\trqqs\r\nrq\r\nrs"
// where:
// "^^^" - one log record seperator (cause multiline request/response values).
// "err" - error message.
// "cmd" - application command.
// "code" - HTTP response code.
// "latency" - request execution time.
// "ip" - client ip.
// "srvc" - 5 char service abbreviation.
// "rqct" - request content type.
// "rsct" - response content type.
// "reqid" - request id (UUID).
// "uid" - user id (UUID).
// "rqqs" - request query string.
// "useragent" - obviously
// "rq" - request body.
// "rs" - response body.
// column separator does matter. "\n" means that column is placed on the new line.
func InitF(logHeaderFormat string) {

	_initialize("", true, logHeaderFormat)

}

// initializes log feature when many services writes to the one server database log
//
// Parameters:
//
// - srvabbr: service abbreviation (5 letters in caps, f.e.: LOGER). Mandatory if isStandalone = false
//
// - isStandalone true - writes to file, writes to logserver (to files if server is inaccesable)
func InitMS(srvabbr string, isStandalone bool) {

	_initialize(srvabbr, isStandalone, "")

}

// initializes log feature
//
// Parameters:
//
// - srvabbr: service abbreviation (5 letters in caps, f.e.: LOGER). Mandatory if isStandalone = false
//
// - isStandalone true - writes to file, writes to logserver (to files if server is inaccesable)
//
// - logHeaderFormat - example: "^^^\tdatetime\terr\tcmd\tcode\tlatency\tip\tsrvc\trqct\trsct\treqid\tuid\trqqs\r\nrq\r\nrs"
// where:
// "^^^" - one log record seperator (cause multiline request/response values).
// "err" - error message.
// "cmd" - application command.
// "code" - HTTP response code.
// "latency" - request execution time.
// "ip" - client ip.
// "srvc" - 5 char service abbreviation.
// "rqct" - request content type.
// "rsct" - response content type.
// "reqid" - request id (UUID).
// "uid" - user id (UUID).
// "rqqs" - request query string.
// "useragent" - obviously
// "rq" - request body.
// "rs" - response body.
// column separator does matter. "\n" means that column is placed on the new line.
func InitMSF(srvabbr string, isStandalone bool, logHeaderFormat string) {

	_initialize(srvabbr, isStandalone, logHeaderFormat)

}

func _initialize(srvabbr string, isStandalone bool, logHeaderFormat string) {

	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("can not create logger instance: ", r)
			os.Exit(1)
		}
	}()

	_isStandalone = isStandalone

	if _rxSrvAbbr.MatchString(srvabbr) {
		_serviceAbbr = srvabbr
	} else {
		if !_isStandalone {
			printError("service name should by 5 letters in caps. F.e.: LOGER")
			os.Exit(1)
		}
	}
	_fileNameFormat = []string{"D", "T"} // default file format

	_infoMessageBuf = &bytes.Buffer{}
	_errorMessageBuf = &bytes.Buffer{}

	var (
		err error
	)

	_logPath, err = getDefaultLogPath()
	if err != nil {
		printError("log folder not defined")
		os.Exit(1)
	}
	fmt.Println("default log path: [" + _logPath + "]")

	_uuidInstanceID = uuid.New()

	if !_isStandalone {
		go initializeClient()
	}

	// validate log file accessability

	// ************************************************
	// !!!!creates log file within docker if it runs there!!!
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

	if len(logHeaderFormat) > 0 {
		err = setLogFileFormat(logHeaderFormat)
	} else {
		err = setLogFileFormat(_log_format)
	}
	if err != nil {
		fmt.Printf("can not parse log file: %s\n", err.Error())
		os.Exit(1)
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

func SetLogFileCountLimit(val int) {

	if val == -1 {
		_logFileCountLimit = val
	} else if val < _logFileCountLimitMin {
		_logFileCountLimit = _logFileCountLimitMin
	} else {
		_logFileCountLimit = val
	}
}

func LogPath() string { return _logPath }

// project name to cut stacktrace on errors
func SetLogMarker(marker string) {
	if len(strings.TrimSpace(marker)) > 0 {
		_log_mark = marker
	}
}

// weblogsrv url
func LogServerURL() string {
	return SERVER_URL
}
func SetLogServerURL(url string) bool {
	url = strings.TrimSpace(url)
	if len(url) == 0 {
		return false
	}
	if _rxUrl.MatchString(url) {
		SERVER_URL = url
	} else {
		if _rxIPv4v6.MatchString(url) {
			SERVER_URL = url
		} else {
			AddError("web server URL is invalid")
			WriteTask()
			return false
		}
	}
	return true
}

// "info" - just query string (default) end response HTTP code
// "trace" - as "debug" but request body and response body are cut up to 1KB
// "debug" - full info: query string, request body, response body, (headers?), errors
// "error" - full info only if error ocuures
func SetLogLevel(level string) bool {

	level = strings.TrimSpace(level)
	if len(level) == 0 {
		return false
	}
	switch strings.ToLower(level) {
	case "info":
		_loglevel = LOG_INFO
		return true
	case "trace":
		_loglevel = LOG_TRACE
		return true
	case "debug":
		_loglevel = LOG_DEBUG
		return true
	case "error":
		_loglevel = LOG_ERROR
		return true
	}
	return false
}

// change log directory, and create new log file in it (if it does not exist).
// if not set, directory "logs" will be created next to the executable.
func SetLogPath(logdir string) bool {

	if len(logdir) == 0 {
		return false
	}

	logdir = strings.TrimSpace(logdir)

	if logdir == _logPath {
		return true
	}

	if len(logdir) == 0 {
		return false
	}
	if !strings.HasSuffix(logdir, _path_separator) {
		logdir = logdir + _path_separator
	}

	err := os.MkdirAll(logdir, os.ModePerm)
	if err != nil {
		fmt.Println("can not cfeate new log directory:" + logdir + "\nERROR: " + err.Error())
		AddError(fmt.Sprintf("can not cfeate new log directory: %s\n%s", logdir, err.Error()))
		WriteTask()
		return false
	}
	_logPath = logdir

	fileLog, _, err := createLogFile(logdir)
	if err != nil {
		fmt.Println("can not cfeate log file in new directory: " + logdir + "\nERROR:" + err.Error())
		AddError(fmt.Sprintf("can not cfeate log file in new directory: %s\n%s", logdir, err.Error()))
		WriteTask()
		return false //printError(strErr)
	}

	_muLogWrite.Lock()
	Close()
	_fileLog = fileLog
	_muLogWrite.Unlock()

	return true
}

// May be used in log file names, when many instances write logs into one folder.
// Every instance writes log into its own file to avoid concurrency problem.
// To use it, add U in SetFileNameFormat() parameter.
// If it is not set, it is generated automatically on initialization.
func SetInstanceID(srvid uuid.UUID) {
	_uuidInstanceID = srvid
}

func InstanceID() uuid.UUID {
	return _uuidInstanceID
}

// A - abbreviation, D - date, T - time, U - instance ID (UUID)
// f.e.:ADTU - SRV01.2022-12-12.15-03-12.b461cc28-8bab-4c19-8e25-f4c17faf5638.log
// by default: 2022-12-12.log
func SetFileNameFormat(fileNameFormat string) bool {
	fileNameFormat = strings.TrimSpace(fileNameFormat)
	if len(fileNameFormat) == 0 {
		return true
	}
	fnf := "D"
	if _rxFileFormat.MatchString(fileNameFormat) {
		fnf = fileNameFormat
	} else {
		AddError("fileNameFormat parameter is invalid: expected \"A\",\"D\" or \"T\" or their combination")
		WriteTask()
		return false
	}
	_fileNameFormat = nil
	for _, r := range fnf {
		_fileNameFormat = append(_fileNameFormat, string(r))
	}

	createLogFileAgain()

	return true
}

// A - abbreviation, D - date, T - time, U - instance ID (UUID)
// f.e.:ADTU - SRV01.2022-12-12.15-03-12.b461cc28-8bab-4c19-8e25-f4c17faf5638.log
// by default: 2022-12-12.log
// period - period recreate log file: minute, hour, day
// periodCount - count of period (10 min)
func SetFileNameFormatExt(fileNameFormat string, period string, periodCount int) bool {
	fileNameFormat = strings.TrimSpace(fileNameFormat)
	if len(fileNameFormat) == 0 {
		return true
	}
	fnf := "D"
	if _rxFileFormat.MatchString(fileNameFormat) {
		fnf = fileNameFormat
	} else {
		AddError("fileNameFormat parameter is invalid: expected \"A\",\"D\" or \"T\" or their combination")
		WriteTask()
		return false
	}
	_fileNameFormat = nil
	for _, r := range fnf {
		_fileNameFormat = append(_fileNameFormat, string(r))
	}

	createLogFileAgain()

	switch {
	case period == "day":
		_period = 111
	case period == "hour":
		_period = 11
	case period == "minute":
		_period = 1
	}

	_periodCount = periodCount
	return true
}
func IsServiceAbbreviation(srvabbr string) bool { return _rxSrvAbbr.MatchString(srvabbr) }

func GetLogFilePath() string {
	if _fileLog == nil {
		return ""
	}
	return _fileLog.Name()
}

// example: "^^^\tdatetime\terr\tcmd\tcode\tlatency\tip\tsrvc\trqct\trsct\treqid\tuid\trqqs\r\nrq\r\nrs"
// "^^^" - one log record seperator (cause multiline request/response values).
// "err" - error message.
// "cmd" - application command.
// "code" - HTTP response code.
// "latency" - request execution time.
// "ip" - client ip.
// "srvc" - 5 char service abbreviation.
// "rqct" - request content type.
// "rsct" - response content type.
// "reqid" - request id (UUID).
// "uid" - user id (UUID).
// "rqqs" - request query string.
// "useragent" - obviously
// "rq" - request body.
// "rs" - response body.
// column separator does matter. "\n" means that column is placed on the new line.
func setLogFileFormat(val string) error {

	if len(val) == 0 {
		return errors.New("empty param")
	}

	_log_format = NormalizeNewlines(val)
	if len(_log_format) == 0 {
		return errors.New("empty param after normalizing")
	}

	// When log a single record on server,
	// empty line is the delimiter between format info and data.
	// If the format info will be placed in the config file,
	// this mandatory feature may be omitted by user.
	if !strings.HasSuffix(_log_format, "\n\n") {
		_log_format = _log_format + "\n\n"
	}
	// header := _log_format[:len(_log_format)-2]
	info, err := parseLogFileHeader(_log_format)
	if err != nil {
		return fmt.Errorf("can not parse log file format: %s", err.Error())
	}

	_rowtags = info.Columns
	_tagDelimiters = info.ColumnDelimiters
	_recordDelimmiter = info.RecordDelimmiter
	if _mapHeaders == nil {
		_mapHeaders = make(map[string]*logFormatInfo)
	}

	_mapHeaders[_log_format] = info

	return nil
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
