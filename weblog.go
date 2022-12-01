package weblog

/*			TODO:
Uid from string to uuid, but latency remained as string
определиться при вствке в базу
*/

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
	// FieldMaxTextLen      = -1
	FieldMaxContentType = 75
)

var (
	_log_mark       string // to reduce stacktrace path
	_path_separator string = string(os.PathSeparator)
	_log_format            = `^^^	datetime	err	cmd	code	latency	ip	srvc	rqct	rsct	reqid	uid	rqqs
rq
rs
`
	_recordDelimmiter string = "^^^"

	_loglevel            int = LOG_INFO
	_muLogWrite              = &sync.Mutex{}
	_fileLog             *os.File
	_lastDay, _lastMonth int = 0, 0
	_logPath             string
	_uuidDefault         uuid.UUID //[16]byte
	_uuidInstanceID      uuid.UUID //[16]byte

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
	_rxIPv4v6 = regexp.MustCompile(`((^\s*((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))\s*$)|(^\s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))(%.+)?\s*$))`)

	// https://www.regextester.com/104038
	// rxIp     = regexp.MustCompile(`((^\s*((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))\s*$)|(^\s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))(%.+)?\s*$))`)

	_infoMessageBuf  *bytes.Buffer
	_errorMessageBuf *bytes.Buffer
	_fileNameFormat  []string = []string{}

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
	time               *time.Time
	timeStr            string
	recid              uuid.UUID
	uid                uuid.UUID
	code               int
	codeStr            string
	cmd                string
	latency            int64
	latencyStr         string
	ip                 string
	rqct               string
	rsct               string
	rqqs               string
	rq                 string
	is_response_binary bool // for images

	Enabled     bool // true by default. Set false not to log
	serviceName string
	loglevel    int

	logRawRecord     string
	responseBuffer   *bytes.Buffer
	stacktraceBuffer *bytes.Buffer
	recRawBuffer     *bytes.Buffer
	mapQS            url.Values
}

func NewLogger() *Logger {

	w := &Logger{}
	w.Enabled = true
	w.responseBuffer = &bytes.Buffer{}
	w.recRawBuffer = &bytes.Buffer{}
	w.responseBuffer.Grow(MAX_INT_10KB)
	w.stacktraceBuffer = &bytes.Buffer{}
	w.stacktraceBuffer.Grow(MAX_INT_10KB)
	w.SetTime(time.Now())
	w.SetResponseCode(200)
	w.rqct = "text/plain"
	w.logRawRecord = ""
	w.loglevel = _loglevel

	return w
}

func (w *Logger) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)

	encoder.Encode(&w.serviceName)
	encoder.Encode(&w.time)
	encoder.Encode(&w.code)
	encoder.Encode(&w.latency)
	encoder.Encode(&w.recid)
	encoder.Encode(&w.uid)
	encoder.Encode(&w.ip)
	encoder.Encode(&w.cmd)
	encoder.Encode(&w.rqct)
	encoder.Encode(&w.rsct)
	encoder.Encode(&w.rqqs)
	encoder.Encode(&w.rq)
	encoder.Encode(w.responseBuffer.String())
	encoder.Encode(w.stacktraceBuffer.String())
	encoder.Encode(w.recRawBuffer.String())
	return buf.Bytes(), nil
}
func (w *Logger) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	decoder.Decode(&w.serviceName)
	decoder.Decode(&w.time)
	decoder.Decode(&w.code)
	decoder.Decode(&w.latency)
	decoder.Decode(&w.recid)
	decoder.Decode(&w.uid)
	decoder.Decode(&w.ip)
	decoder.Decode(&w.cmd)
	decoder.Decode(&w.rqct)
	decoder.Decode(&w.rsct)
	decoder.Decode(&w.rqqs)
	decoder.Decode(&w.rq)
	var str string
	w.responseBuffer = &bytes.Buffer{}
	w.recRawBuffer = &bytes.Buffer{}
	w.responseBuffer.Grow(MAX_INT_10KB)
	w.stacktraceBuffer = &bytes.Buffer{}
	decoder.Decode(&str)
	w.responseBuffer.WriteString(str)
	decoder.Decode(&str)
	w.stacktraceBuffer.WriteString(str)
	decoder.Decode(&str)
	w.recRawBuffer.WriteString(str)
	str = ""
	return nil
}

func (w *Logger) SetTime(val time.Time) {
	w.time = &val
	w.timeStr = val.Format(TIME_LAYOUT)
}
func (w *Logger) SetTimeStr(val string) {
	if len(val) == 0 {
		return
	}
	t, err := time.Parse(TIME_LAYOUT, val)
	if err == nil {
		w.time = &t
		w.timeStr = val
	} else {
		w.stacktraceBuffer.WriteString(
			joinString("datetime: ", err.Error(), NEW_LINE))
	}
}
func (w *Logger) Time() *time.Time {
	return w.time
}

func (w *Logger) IsResposeBinary() bool {
	return w.is_response_binary
}

func (w *Logger) SetServiceName(val string) {

	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if _rxSrvAbbr.MatchString(val) {
		w.serviceName = val
	} else {
		w.stacktraceBuffer.WriteString("service name should by 5 letters in caps. F.e.: LOGER")
	}
}
func (w *Logger) ServiceName() string {
	return w.serviceName
}

func (w *Logger) SetCommand(val string) {

	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldCmdLen {

		if _rxCmd.MatchString(val) {
			w.cmd = val
		} else {
			w.stacktraceBuffer.WriteString(joinString(
				"can not parse command", NEW_LINE))
		}
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"command length exceeded: ",
			strconv.Itoa(len(val)), " of ", strconv.Itoa(FieldCmdLen), TAB, val[:FieldCmdLen-1], NEW_LINE))
	}
}

func (w *Logger) Command() string {
	return w.cmd
}

func (w *Logger) SetResponseCode(val int) { w.code = val }
func (w *Logger) SetResponseCodeStr(val string) {
	if len(val) == 0 {
		return
	}
	intTmp, err := strconv.Atoi(val)
	if err == nil {
		w.code = intTmp
		w.codeStr = val
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"response code: ", err.Error(), NEW_LINE))
	}
}
func (w *Logger) ResponseCode() int { return w.code }

func (w *Logger) SetLatency(val int64) {
	if val < 0 {
		return
	}
	w.latency = int64(val)
	w.latencyStr = strconv.FormatInt(w.latency, 10)
	// w.latencyStr =  strconv.FormatFloat(w.latency, 'f', 3, 64)
}
func (w *Logger) SetLatencyStr(val string) {
	if len(val) == 0 {
		return
	}
	// val = strings.Replace(val, ".", "", -1)
	l, err := strconv.Atoi(val)
	// l, err := strconv.ParseFloat(val, 64)
	if err == nil {
		w.latency = int64(l)
		w.latencyStr = val
		// w.latencyStr = strconv.FormatFloat(l, 'f', 2, 64)
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"latency: ", err.Error(), NEW_LINE))
	}
}
func (w *Logger) Latency() int64 { return w.latency }

func (w *Logger) SetIP(val string) {
	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= 39 {
		w.ip = val
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"ip is longer then 39 (max ipv6 length). Length: ",
			strconv.Itoa(intTmp), TAB, val, NEW_LINE))
	}
}
func (w *Logger) IP() string { return w.ip }

func (w *Logger) SetRequestContentType(val string) {
	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldMaxContentType {
		w.rqct = val
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"rqct is longer then ",
			strconv.Itoa(FieldMaxContentType), " : ", strconv.Itoa(intTmp), NEW_LINE))
	}
}
func (w *Logger) RequestContentType() string { return w.ip }

func (w *Logger) SetResponseContentType(val string) {
	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldMaxContentType {
		w.rsct = val
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"rsct is longer then ",
			strconv.Itoa(FieldMaxContentType), " : ", strconv.Itoa(intTmp), NEW_LINE))
	}
}
func (w *Logger) ResponseContentType() string { return w.rsct }

func (w *Logger) SetRequestId(val uuid.UUID) { w.recid = val }
func (w *Logger) SetRequestIdStr(val string) {
	if len(val) == 0 {
		return
	}
	uuidTmp, err := uuid.Parse(val)
	if err == nil {
		w.recid = uuidTmp
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"reqid: ", err.Error(), NEW_LINE))
	}
}
func (w *Logger) RequestId() uuid.UUID { return w.recid }

func (w *Logger) SetUserId(val uuid.UUID) { w.uid = val }
func (w *Logger) SetUserIdStr(val string) {
	if len(val) == 0 {
		return
	}
	uuidTmp, err := uuid.Parse(val)
	if err == nil {
		w.uid = uuidTmp
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"uid: ", err.Error(), NEW_LINE))
	}
}
func (w *Logger) UserId() uuid.UUID { return w.uid }

func (w *Logger) SetRequestQS(val string) {
	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldMaxRequestQSLen {
		w.rqqs = val
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"rqqs is longer then ",
			strconv.Itoa(FieldMaxRequestQSLen), " : ", strconv.Itoa(intTmp), NEW_LINE))
	}
}

func (w *Logger) SetRequestQSMap(qs url.Values) { w.mapQS = qs }

func (w *Logger) RequestQS() string { return w.rqqs }

func (w *Logger) RequestQSGetValue(key string) string { return w.mapQS.Get(key) }

func (w *Logger) SetRequest(val string) {

	// intTmp := len(val)
	// if intTmp == 0 {
	// 	return
	// }
	w.rq = val // logger may be used as dto

	// if FieldMaxTextLen > -1 {
	// 	if intTmp <= FieldMaxTextLen {
	// 		w.rq = val
	// 	} else {
	// 		w.stacktraceBuffer.WriteString(joinString(
	// 			"rq is longer then ",
	// 			strconv.Itoa(FieldMaxTextLen), " : ", strconv.Itoa(intTmp), NEW_LINE))
	// 	}
	// } else {
	// 	w.rq = val
	// }
}
func (w *Logger) Request() string { return w.rq }

func (w *Logger) SetResponse(val string) { w.AddResponse(val) }
func (w *Logger) Response() string       { return w.responseBuffer.String() }
func (w *Logger) ResponseBinary() []byte { return w.responseBuffer.Bytes() }
func (w *Logger) HasResponse() bool      { return w.responseBuffer.Len() > 0 }

func (w *Logger) LogLevel() string {
	switch w.loglevel {
	case LOG_INFO:
		return "info"
	case LOG_TRACE:
		return "trace"
	case LOG_DEBUG:
		return "debug"
	case LOG_ERROR:
		return "error"
	default:
		return "info"
	}
}

// "info" - just query string (default) end response HTTP code
// "trace" - as "debug" but request body and response body are cut up to 1KB
// "debug" - full info: query string, request body, response body, (headers?), errors
// "error" - full info only if error ocuures
func (w *Logger) SetLogLevel(level string) {

	switch level {
	case "info":
		w.loglevel = LOG_INFO
	case "trace":
		w.loglevel = LOG_TRACE
	case "debug":
		w.loglevel = LOG_DEBUG
	case "error":
		w.loglevel = LOG_ERROR
	}
}

// clears all fields and sets buffers to nil
func (w *Logger) Reset() {

	w.Clear()
	w.responseBuffer = nil
	w.recRawBuffer = nil
	w.stacktraceBuffer = nil
}

// clears all fields
func (w *Logger) Clear() {

	w.code = 0
	w.cmd = ""
	w.codeStr = ""
	w.ip = ""
	w.latency = 0
	w.latencyStr = ""
	w.mapQS = nil
	w.recid = uuid.Nil
	w.uid = uuid.Nil
	w.rq = ""
	w.rqct = ""
	w.rsct = ""
	w.rqqs = ""
	w.time = nil
	w.timeStr = ""

	w.Enabled = false
	w.logRawRecord = ""
	w.responseBuffer.Reset()
	w.stacktraceBuffer.Reset()
	w.recRawBuffer.Reset()

}

// FUNCTIONS

func (w *Logger) WriteRequest() {

	var (
		iserr       = w.stacktraceBuffer.Len() > 0
		strResponse string
		strReq      string
	)

	if w.loglevel == LOG_ERROR && !iserr {
		return
	}

	sb := &bytes.Buffer{}
	// // this has to be ordered by logging data size
	// LOG_ERROR = 1 "error" - full info only if error ocuures
	// LOG_INFO  = 2 just query string (default) end response HTTP code
	// LOG_TRACE = 3 as "debug" but request body and response body are cut up to 1KB
	// LOG_DEBUG = 4 full info: query string, request body, response body, (headers?), errors

	strErr := ""
	intErrorLen := 0
	if iserr {
		sb.Grow(w.stacktraceBuffer.Len())
		strErr = w.stacktraceBuffer.String()
		intErrorLen = len(strErr)
	}

	if !iserr && w.loglevel != LOG_INFO {
		strReq = w.Request()
		if !iserr && w.loglevel == LOG_TRACE {
			if len(strReq) > MAX_INT_1KB {
				sb.Grow(MAX_INT_1KB)
				sb.WriteString(strReq[:800])
				sb.WriteString(NEW_LINE)
				sb.WriteString("...")
				sb.WriteString(NEW_LINE)
				sb.WriteString(strReq[len(strReq)-200:])
				w.SetRequest(sb.String())
				sb.Reset()
			}
		} else {
			w.SetRequest(strReq)
		}
		strResponse = w.responseBuffer.String()
		intResponseLen := len(strResponse)
		if !iserr && w.loglevel == LOG_TRACE && intResponseLen > MAX_INT_1KB {
			sb.Grow(MAX_INT_1KB)
			sb.WriteString(strResponse[:800])
			sb.WriteString(NEW_LINE)
			sb.WriteString("...")
			sb.WriteString(NEW_LINE)
			sb.WriteString(strResponse[intResponseLen-200:])
			strResponse = sb.String()
			w.SetResponse(strResponse)
			sb.Reset()
		} else {
			w.SetResponse(strResponse)
		}
	}

	strIP := w.ip
	intLen := len(w.timeStr) + len(w.latencyStr) + len(strIP) + len(w.cmd) +
		len(strReq) + len(strResponse) + intErrorLen + 100
	if intLen > sb.Cap() {
		sb.Grow(intLen)
	}

	// _lblError

	sb.WriteString(_recordDelimmiter)
	sb.WriteString(_tagDelimiters[0])
	// `time err cmd code latency ip rqct rsct reqid uid rqqs
	// rq
	// rs`
	for i, tab := range _rowtags {
		switch tab {
		case "datetime":
			sb.WriteString(w.timeStr)
		case "err":
			if w.code >= 500 {
				sb.WriteString(LBL_ERROR)
			}
		case "cmd":
			sb.WriteString(w.cmd)
		case "code":
			sb.WriteString(strconv.Itoa(w.code))
		case "latency":
			sb.WriteString(w.latencyStr)
		case "ip":
			sb.WriteString(strIP)
		case "srvc":
			sb.WriteString(_serviceAbbr)
		case "rqct":
			sb.WriteString(w.rqct)
		case "rsct":
			sb.WriteString(w.rsct)
		case "recid":
			if w.recid != _uuidDefault {
				sb.WriteString(w.recid.String())
			}
		case "uid":
			if w.uid != _uuidDefault {
				sb.WriteString(w.uid.String())
			}
		case "rqqs":
			sb.WriteString(w.rqqs)
		case "rq":
			sb.WriteString("rq:")
			sb.WriteString(w.rq)
		case "rs":
			sb.WriteString("rs:")
			if intErrorLen == 0 {
				sb.WriteString(strResponse)
			} else {
				sb.WriteString(strErr)
			}
		}
		sb.WriteString(_tagDelimiters[i])
	}

	res := sb.String()
	sb.Reset()
	logTxt(&res)
	res = ""
}

func (w *Logger) HasRequestParseError() bool { return w.stacktraceBuffer.Len() > 0 }

func (w *Logger) RequestParseError() string { return w.stacktraceBuffer.String() }

// everything that user will get
func (w *Logger) AddResponse(val string) {

	sz := len(val)
	if sz == 0 {
		return
	}
	buf := w.responseBuffer
	if sz > buf.Cap() {
		buf.Grow(sz)
	}
	buf.WriteString(val)
	// buf.WriteString(NEW_LINE)
}

// f.e. images
func (w *Logger) AddResponseBinary(val []byte) {

	w.is_response_binary = true
	sz := len(val)
	if sz == 0 {
		return
	}
	buf := w.responseBuffer
	buf.Reset()
	buf.Write(val)
}

// for the log only, not for the client
func (w *Logger) AddStacktrace(errmsg string) *Logger {

	buf := w.stacktraceBuffer
	// buf.WriteString(NewLine)
	if len(errmsg) > 0 {
		buf.WriteString(errmsg)
		buf.WriteString(NEW_LINE)
	}
	printStackTrace(buf)
	buf.WriteString(NEW_LINE)
	return w
}

// for the log only, not for the client
func (w *Logger) RawData() string { return w.recRawBuffer.String() }

// recovering from a panic during query execution
// without exiting the application
// stack trace should not be included in response
// usage: log.Recover(recover())
func (w *Logger) Recover(r interface{}) {

	if r == nil {
		return
	}
	switch r.(type) {
	case string:
		w.AddStacktrace(r.(string))
	case error:
		w.AddStacktrace(r.(error).Error())
	default:
		w.AddStacktrace("unknown request fatal error")
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
	switch level {
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
		return true
	}

	logdir = strings.TrimSpace(logdir)

	if len(logdir) == 0 {
		return true
	}
	if !strings.HasSuffix(logdir, _path_separator) {
		logdir = logdir + _path_separator
	}

	err := os.MkdirAll(logdir, os.ModePerm)
	if err != nil {
		AddError(fmt.Sprintf("can not cfeate new log directory: %s\n%s", logdir, err.Error()))
		WriteTask()
		return false
	}

	fileLog, _, err := createLogFile(&logdir)
	if err != nil {
		AddError(fmt.Sprintf("can not cfeate log file in new directory: %s\n%s", logdir, err.Error()))
		WriteTask()
		return false //printError(strErr)
	}

	_muLogWrite.Lock()
	if _fileLog != nil {

		_ = _fileLog.Close()
		archihveFile()
		_fileLog = fileLog
	}
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
	return true
}
func IsServiceAbbreviation(srvabbr string) bool { return _rxSrvAbbr.MatchString(srvabbr) }

func GetLogFilePath() string {
	if _fileLog == nil {
		return ""
	}
	return _fileLog.Name()
}

// START | STOP

// initializes log feature
//
// Parameters:
//
// - srvabbr: service abbreviation (5 letters in caps, f.e.: LOGER). Mandatory if isStandalone = false
//
// - isStandalone true - writes to file, writes to logserver (to files if server is inaccesable)
func Initialize(srvabbr string, isStandalone bool) {

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

	_fileNameFormat = []string{"D"} // default file format
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

	_uuidInstanceID = uuid.New()

	if !_isStandalone {
		go initializeClient()
	}

	// validate log file accessability
	var fileExisted = false
	_fileLog, fileExisted, err = createLogFile(&_logPath)
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
		fmt.Printf("can not parse log file: %s\n", err.Error())
		os.Exit(1)
	}
	_rowtags = info.Columns
	_tagDelimiters = info.ColumnDelimiters
	_recordDelimmiter = info.RecordDelimmiter
	_mapHeaders = make(map[string]*logFormatInfo)
	_mapHeaders[_log_format] = info

	_isInitialized = true

}

// on application start / stop
func AddInfo(val string) {
	sz := len(val)
	if sz == 0 {
		return
	}
	buf := _infoMessageBuf
	buf.Grow(sz)
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
	buf.Grow(sz + 2)
	buf.WriteString(val)
	buf.WriteString(NEW_LINE)
}

// on application start / stop
var _latency int64

// latency - service start time
func WriteStart(latency int64) *Logger {

	for i := 0; i < 10; i++ {
		if !_isWebLogSrvAvailable {
			time.Sleep(300 * time.Millisecond)
		} else {
			break
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

func Close() {
	if _fileLog != nil {
		_fileLog.Sync()
		_fileLog.Close()
		archihveFile()
		_fileLog = nil
	}
	_isAppLive = false
	time.Sleep(SLEEP_PING_PERIOD_SEC * time.Second) // wait background cycle exit
}

// PARSING FUNCTIONS

// Loads string columns and values in Logger object
// To avoid key-value map creation on every log record, arrays are used.
// cols array is created the once, before parsing the log file.
// vals - every record values
func ParseLogRecordData(cols []string, vals []string) *Logger {

	lenCols := len(cols)
	if lenCols == 0 || len(vals) == 0 {
		return nil
	}
	// if lenCols != len(vals) {
	// 	return nil
	// }
	rec := NewLogger()

	for i := 0; i < lenCols; i++ {
		switch cols[i] {
		case "datetime":
			rec.SetTimeStr(vals[i])
		// case "err":
		case "cmd":
			rec.SetCommand(vals[i])
		case "code":
			rec.SetResponseCodeStr(vals[i])
		case "srvc":
			rec.SetServiceName(vals[i])
		case "latency":
			rec.SetLatencyStr(vals[i])
		case "ip":
			rec.SetIP(vals[i])
		case "rqct":
			rec.SetRequestContentType(vals[i])
		case "rsct":
			rec.SetResponseContentType(vals[i])
		case "recid":
			rec.SetRequestIdStr(vals[i])
		case "uid":
			rec.SetUserIdStr(vals[i])
		case "rqqs":
			rec.SetRequestQS(vals[i])
		case "rq":
			rec.SetRequest(vals[i])
		case "rs":
			rec.SetResponse(vals[i])
		default:
		}
	}

	// on parsing error save source log record:
	if rec.stacktraceBuffer.Len() > 0 {
		intLen := 0
		for _, v := range vals {
			intLen = intLen + len(v)
		}
		rec.recRawBuffer.Grow(intLen + 100)
		for _, v := range vals {
			rec.recRawBuffer.WriteString(v)
			rec.recRawBuffer.WriteString(NEW_LINE)
		}
	}
	return rec
}

// Parses just one log record with format header
func ParseLogRecordString(logRec *string, returnObjectOnError bool) (*Logger, error) {

	/*
		ONE CLIENT LOG RECORD EXAMPLE:
		(header is devided from record by empty line)

		`^^^ datetime err cmd code latency ip rqct rsct reqid uid rqqs
		rq
		rs

		^^^	2022-06-08 10:41:08.897867		object.action	200	12	127.0.0.1	text/plain	application/json			qqq=wwww&eee=rrr
		rq:some
			multiline
			request
		rs:some
			multiline
			response`
	*/
	logRecord := *logRec
	var (
		idx        int
		line       string
		arrVals    []string
		oLogHeader *logFormatInfo
		ok         bool
		err        error
	)

	// extract log format info

	idx = strings.Index(logRecord, "\n\n")
	if idx == -1 {
		return nil, errors.New("log format info not found in record")
	}
	header := logRecord[:idx+1]

	// if header was parsed earlier, just atke it
	if oLogHeader, ok = _mapHeaders[header]; !ok {
		oLogHeader, err = parseLogFileHeader(header)
		if oLogHeader == nil || err != nil {
			return nil, errors.New("log record header error: " + err.Error())
		}
		_mapHeaders[header] = oLogHeader
	}

	// extract log record

	record := logRecord[idx+2:]
	if len(strings.TrimSpace(record)) == 0 {
		return nil, errors.New("no log data")
	}
	arr := strings.Split(record, "\n")
	sb := &bytes.Buffer{}
	sb.Grow(MAX_INT_10KB)
	idxTag := -1
	for _, line = range arr {
		if strings.HasPrefix(line, oLogHeader.RecordDelimmiter) {
			line = line[len(oLogHeader.RecordDelimmiter)+len(oLogHeader.ColumnDelimiters[0]):]
			// last delimiter \n was removed on split. Retern it back with usual line delimiter
			line += oLogHeader.ColumnDelimiters[0]
			arrVals = strings.Split(line, oLogHeader.ColumnDelimiters[0])
			idxTag = len(arrVals)
		} else {
			if len(arrVals) == 0 {
				if len(line) == 0 {
					continue
				}
				return nil, errors.New("no log record")
			}
			if strings.HasPrefix(line, oLogHeader.Columns[idxTag]) {
				if sb.Len() > 0 {
					arrVals = append(arrVals, sb.String())
					sb.Reset()
				}
				sb.WriteString(line)
				sb.WriteString(NEW_LINE)
				if idxTag < len(oLogHeader.Columns)-1 {
					idxTag++
				}
			} else {
				sb.WriteString(line)
				sb.WriteString(NEW_LINE)
			}
		}
	}
	arrVals = append(arrVals, sb.String())
	sb.Reset()
	sb = nil

	oLog := ParseLogRecordData(oLogHeader.Columns, arrVals)

	if oLog.HasRequestParseError() {
		if returnObjectOnError {
			return oLog, errors.New(oLog.RequestParseError())
		}
		return nil, errors.New(oLog.RequestParseError())
	}
	return oLog, nil

}

// PRIVATE

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
	return nil
}

// in case, that a log file format is not predefined (f.e. it is defined in configuration)
func parseLogFileHeader(format string) (*logFormatInfo, error) {

	info := &logFormatInfo{}

	format = strings.TrimSpace(format)
	if len(format) == 0 {
		return info, errors.New("log file format is missed")
	}

	arrLines := _rxLines.Split(format, -1)
	// arrStr := strings.Split(format, "\n")
	if len(arrLines) == 0 {
		return info, errors.New("log file format is empty")
	}

	tabs := _rxSpaces.Split(arrLines[0], -1)
	if len(tabs) == 0 {
		return info, errors.New("log file format: no columns found")
	}

	// `^^^ err time cmd code latency ip rqct rsct reqid uid rqqs
	// rq
	// rs`
	info.RecordDelimmiter = tabs[0]

	var str string

	for _, tab := range tabs {
		str = strings.TrimSpace(tab)
		if len(str) <= 25 && _rxTag.MatchString(str) {
			info.Columns = append(info.Columns, str)
			info.ColumnDelimiters = append(info.ColumnDelimiters, TAB)
		}
	}
	if len(info.ColumnDelimiters) > 0 {
		info.ColumnDelimiters[len(info.ColumnDelimiters)-1] = NEW_LINE
	}

	for _, tab := range arrLines {
		str = strings.TrimSpace(tab)
		if len(str) <= 25 && _rxTag.MatchString(str) {
			info.Columns = append(info.Columns, str)
			info.ColumnDelimiters = append(info.ColumnDelimiters, NEW_LINE)
		}
	}

	if len(info.Columns) == 0 || len(info.ColumnDelimiters) == 0 {
		return info, errors.New("log file format: no tags found")
	}

	err := info.validate()
	if err == nil {
		return info, err
	} else {
		return nil, err
	}

}

func logTxt(msg *string) {

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
				_fileLog.Close()
				archihveFile()
				_fileLog = nil
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

	var strErr string
	fileLog, _, err := createLogFile(&_logPath)
	if err != nil {
		strErr = fmt.Sprintf("can not cfeate log file: %s\n%s", _logPath, err.Error())
		AddError(strErr)
		WriteTask()
		return printError(strErr)
	}
	_muLogWrite.Lock()
	if _fileLog != nil {
		_ = _fileLog.Close()
		archihveFile()
		_fileLog = fileLog
	}
	_muLogWrite.Unlock()
	return nil
}

// creates new log file
// params: log directory
// returns:
// - file reference,
// - flag, if the file has already been,
// - error
func createLogFile(logpath *string) (*os.File, bool, error) {

	var (
		err              error
		fileLog          *os.File
		fileExistsBefore bool
	)
	if logpath == nil {
		return nil, false, nil
	}

	currTime := time.Now()

	sb := &bytes.Buffer{}
	sb.Grow(1000)
	joinStringBuffP(sb, *logpath)
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

	if !fileExistsBefore {
		_, err = fileLog.WriteString(_log_format) // log file write check
		if err != nil {
			fmt.Println(err.Error())
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

	ex, err := os.Executable()
	if err != nil {
		fmt.Printf("can not get current path:  %s\n", err.Error())
		return "", err
	}
	_logPathDefault := filepath.Dir(ex) + string(os.PathSeparator) + "logs" + string(os.PathSeparator)
	err = os.MkdirAll(_logPathDefault, os.ModePerm)
	if err != nil {
		return "", printError(fmt.Sprintf("can not create folder: %s\n%s", _logPathDefault, err.Error()))
	}
	return _logPathDefault, nil
}

func write(mode int) *Logger {

	sb := &bytes.Buffer{}
	strInfo := _infoMessageBuf.String()
	intInfo := len(strInfo)

	strErr := _errorMessageBuf.String()
	intErr := len(strErr)

	intLen := intInfo + intErr
	if intLen > sb.Cap() {
		sb.Grow(intLen)
	}

	logger := NewLogger()

	sb.WriteString(_recordDelimmiter)
	sb.WriteString(_tagDelimiters[0])
	// `time err cmd code latency ip srvc rqct rsct reqid uid rqqs
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
		case "ip", "rqct", "rsct", "recid", "uid", "rqqs":
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

func archihveFile() {

	fi, err := os.Stat(_fileLog.Name())
	if err != nil {
		return
	}
	diff := int(fi.Size()) - len(_log_format)
	if -10 < diff && diff < 10 {
		os.Remove(_fileLog.Name())
		return
	}

	fdir, fname := filepath.Split(_fileLog.Name())
	err = os.Rename(_fileLog.Name(), fdir+"_"+fname)
	if err != nil {
		AddError(err.Error())
		WriteTask()
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
