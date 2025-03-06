package weblog

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (w *Logger) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)

	encoder.Encode(&w.serviceName)
	encoder.Encode(&w.time)
	encoder.Encode(&w.code)
	encoder.Encode(&w.latency)
	encoder.Encode(&w.reqid)
	encoder.Encode(&w.uid)
	encoder.Encode(&w.ip)
	encoder.Encode(&w.cmd)
	encoder.Encode(&w.rqct)
	encoder.Encode(&w.rsct)
	encoder.Encode(&w.rqqs)
	// encoder.Encode(&w.rq)
	encoder.Encode(w.requestBuffer.String())
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
	decoder.Decode(&w.reqid)
	decoder.Decode(&w.uid)
	decoder.Decode(&w.ip)
	decoder.Decode(&w.cmd)
	decoder.Decode(&w.rqct)
	decoder.Decode(&w.rsct)
	decoder.Decode(&w.rqqs)
	// decoder.Decode(&w.rq)
	var str string
	w.requestBuffer = &bytes.Buffer{}
	w.responseBuffer = &bytes.Buffer{}
	w.recRawBuffer = &bytes.Buffer{}
	w.requestBuffer.Grow(MAX_INT_10KB)
	w.responseBuffer.Grow(MAX_INT_10KB)
	w.stacktraceBuffer = &bytes.Buffer{}

	decoder.Decode(&str)
	w.requestBuffer.WriteString(str)
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

// useragent  string
func (w *Logger) SetUserAgent(val string) {
	if len(val) > 255 {
		val = val[:255]
	}
	val = _rxUserAgent.ReplaceAllString(val, "")
	w.useragent = val
}

// To reduce log size UserAgent is logged only when this command occures
func (w *Logger) SetUserAgentCommand(val string) {
	w.useragent_cmd = val
}

func (w *Logger) SetLanguage(val string) { w.lang = val }

func (w *Logger) SetContext(val context.Context) { w.ctx = val }

func (w *Logger) Language() string { return w.lang }
func (w *Logger) Context() context.Context {
	if w.ctx == nil {
		w.ctx = context.Background()
	}
	return w.ctx
}

func (w *Logger) SetCommand(val string) {

	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldCmdLen {

		if _rxCmd.MatchString(val) {
			w.cmd = val
			// } else {
			// 	w.stacktraceBuffer.WriteString(joinString(
			// 		"can not parse command", NEW_LINE))
		}
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"command length exceeded: ",
			strconv.Itoa(len(val)), " of ", strconv.Itoa(FieldCmdLen), TAB, val[:FieldCmdLen-1], NEW_LINE))
	}
}
func (w *Logger) SetCommandIfExists(val string) {

	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldCmdLen && _rxCmd.MatchString(val) {
		w.cmd = val
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

	if strings.Contains(val, ",") {
		sb := &bytes.Buffer{}
		for _, strip := range strings.Split(val, ",") {
			if _rxIPv4v6_simple.MatchString(strip) {
				sb.WriteString(strip)
				sb.WriteString(",")
			}
		}
		val = string((sb.Bytes()[:sb.Len()-1]))
		sb.Reset()
		sb = nil
	} else {
		if !_rxIPv4v6_simple.MatchString(val) {
			w.ip = "0.0.0.0"
		}
	}
	if len(val) > FieldMaxIPLength {
		val = val[:FieldMaxIPLength]
	}
	w.ip = val
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
		w.rqct = val[:FieldMaxContentType]
		// w.stacktraceBuffer.WriteString(joinString(
		// 	"rqct is longer then ",
		// 	strconv.Itoa(FieldMaxContentType), " : ", strconv.Itoa(intTmp), NEW_LINE))
	}
}
func (w *Logger) RequestContentType() string { return w.rqct }

func (w *Logger) SetResponseContentType(val string) {
	intTmp := len(val)
	if intTmp == 0 {
		return
	}
	if intTmp <= FieldMaxContentType {
		w.rsct = val
	} else {
		w.rsct = val[:FieldMaxContentType]
		// w.stacktraceBuffer.WriteString(joinString(
		// 	"rsct is longer then ",
		// 	strconv.Itoa(FieldMaxContentType), " : ", strconv.Itoa(intTmp), NEW_LINE))
	}
}
func (w *Logger) ResponseContentType() string { return w.rsct }

func (w *Logger) SetRequestId(val uuid.UUID) { w.reqid = val }
func (w *Logger) SetRequestIdStr(val string) {
	if len(val) == 0 {
		return
	}
	uuidTmp, err := uuid.Parse(val)
	if err == nil {
		w.reqid = uuidTmp
	} else {
		w.stacktraceBuffer.WriteString(joinString(
			"reqid: ", err.Error(), NEW_LINE))
	}
}
func (w *Logger) RequestId() uuid.UUID { return w.reqid }

func (w *Logger) SetUser(val interface{}) { w.user = val }
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
func (w *Logger) User() interface{} { return w.user }
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

func (w *Logger) IsRequestBinary() bool {
	return w.is_request_binary
}

// deprecated. See SystemError() property
func (w *Logger) Error() string {
	return w.stacktraceBuffer.String()
}

// it is set by application. Because there is no reliable way to define if request is binary,
// application function should set this flag explicitly. False by default.
func (w *Logger) SetIsRequestBinary(val bool) {
	w.is_request_binary = val
}
func (w *Logger) SetRequest(val []byte) {

	sz := len(val)
	if sz == 0 {
		return
	}
	buf := w.requestBuffer
	buf.Reset()
	if sz > buf.Cap() {
		buf.Grow(sz)
	}
	buf.Write(val)
}

// add additional info log
func (w *Logger) AddToLog(val string) {

	sz := len(val)
	if sz == 0 {
		return
	}
	buf := w.responseBuffer
	buf.WriteString("\n")
	buf.WriteString(val)
	buf.WriteString("\n")
}

// add additional info log
func (w *Logger) AddToLogError(val string) {

	sz := len(val)
	if sz == 0 {
		return
	}
	buf := w.stacktraceBuffer
	buf.WriteString("\n")
	buf.WriteString(val)
	buf.WriteString("\n")

}

// func (w *Logger) SetRequest(val string) {

// 	// intTmp := len(val)
// 	// if intTmp == 0 {
// 	// 	return
// 	// }

// 	w.rq = val // logger may be used as dto

//		// if FieldMaxTextLen > -1 {
//		// 	if intTmp <= FieldMaxTextLen {
//		// 		w.rq = val
//		// 	} else {
//		// 		w.stacktraceBuffer.WriteString(joinString(
//		// 			"rq is longer then ",
//		// 			strconv.Itoa(FieldMaxTextLen), " : ", strconv.Itoa(intTmp), NEW_LINE))
//		// 	}
//		// } else {
//		// 	w.rq = val
//		// }
//	}
func (w *Logger) Request() []byte { return w.requestBuffer.Bytes() }

func (w *Logger) SetResponse(val string) { w.AddResponse(val) }

// f.e. images (no accumulation)
func (w *Logger) SetResponseBinary(val []byte) {

	w.is_response_binary = true
	sz := len(val)
	if sz == 0 {
		return
	}
	buf := w.responseBuffer
	buf.Reset()
	remains := buf.Cap() - buf.Len()
	if sz > remains {
		buf.Grow(sz + buf.Len())
	}
	buf.Write(val)
}

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
	w.reqid = uuid.Nil
	w.uid = uuid.Nil
	// w.rq = ""
	w.rqct = ""
	w.rsct = ""
	w.rqqs = ""
	w.time = nil
	w.timeStr = ""

	w.Enabled = false
	// w.logRawRecord = ""
	w.requestBuffer.Reset()
	w.responseBuffer.Reset()
	w.stacktraceBuffer.Reset()
	w.recRawBuffer.Reset()

}

// FUNCTIONS

// writes request data to log after app execution
func (w *Logger) WriteRequest(clearBase64 bool) {

	var (
		iserr       = w.stacktraceBuffer.Len() > 0
		strResponse string
		strReq      string
	)

	if w.loglevel == LOG_ERROR && !iserr {
		return
	}

	sb := &bytes.Buffer{}
	sb.Grow(MAX_INT_1KB)
	// // this has to be ordered by logging data size
	// LOG_ERROR = 1 "error" - full info only if error ocuures
	// LOG_INFO  = 2 just query string (default) end response HTTP code
	// LOG_TRACE = 3 as "debug" but request body and response body are cut up to 1KB
	// LOG_DEBUG = 4 full info: query string, request body, response body, (headers?), errors

	strErr := ""
	intErrorLen := 0
	if iserr {
		if w.stacktraceBuffer.Len() > MAX_INT_1KB {
			sb.Grow(w.stacktraceBuffer.Len())
		}
		strErr = w.stacktraceBuffer.String()
		intErrorLen = len(strErr)
	}
	var (
		intResponseLen int
		intRequestLen  int
	)
	if w.loglevel == LOG_TRACE && len(w.cmd) > 0 {

		// request
		if !w.is_request_binary {

			if clearBase64 {
				strReq = RemoveBase64Content(w.requestBuffer.String())
			} else {
				strReq = w.requestBuffer.String()
			}
			intRequestLen = len(strReq)

			if !iserr && w.loglevel == LOG_TRACE && intRequestLen > MAX_INT_1KB {
				sb.Grow(sb.Cap() + MAX_INT_1KB)
				sb.WriteString(strReq[:800])
				sb.WriteString(NEW_LINE)
				sb.WriteString("...")
				sb.WriteString(NEW_LINE)
				sb.WriteString(strReq[len(strReq)-200:])
				intRequestLen = sb.Len()
				strReq = sb.String()
				sb.Reset()
			} else {
				intRequestLen = len(strReq)
				// strReq = string(w.Request())
			}
		} else {
			str := "binary request. Size: " + strconv.Itoa(len(w.Request()))
			intRequestLen = len(str)
			strReq = str
		}

		// response
		if !w.is_response_binary {

			if clearBase64 {
				strResponse = RemoveBase64Content(w.responseBuffer.String())
			} else {
				strResponse = w.responseBuffer.String()
			}
			intResponseLen = len(strResponse)

			// !iserr && removed because full html returned on 403
			if w.loglevel == LOG_TRACE && intResponseLen > MAX_INT_1KB {
				sb.Grow(MAX_INT_1KB)
				sb.WriteString(strResponse[:800])
				sb.WriteString(NEW_LINE)
				sb.WriteString("...")
				sb.WriteString(NEW_LINE)
				sb.WriteString(strResponse[intResponseLen-200:])
				intResponseLen = sb.Len()
				strResponse = sb.String()
				sb.Reset()
			} else {
				intResponseLen = w.responseBuffer.Len()
				// strResponse = w.responseBuffer.String()
			}
		} else {
			str := "binary response. Size: " + strconv.Itoa(len(w.ResponseBinary()))
			intResponseLen = len(str)
			strResponse = str
		}

	} else if (w.loglevel == LOG_ERROR || w.loglevel == LOG_DEBUG) && len(w.cmd) > 0 {

		if !w.is_request_binary {

			intRequestLen = w.requestBuffer.Len()
			strReq = w.requestBuffer.String()

		} else {

			strReq := strconv.Itoa(intRequestLen)
			intRequestLen = len(strReq)
		}

		if !w.is_response_binary {

			intResponseLen = w.responseBuffer.Len()
			strResponse = w.responseBuffer.String()

		} else {

			strResponse := strconv.Itoa(intResponseLen)
			intResponseLen = len(strResponse)

		}

	} else { // INFO

		strReq := strconv.Itoa(intRequestLen)
		strResponse := strconv.Itoa(intResponseLen)
		intRequestLen = len(strReq)
		intResponseLen = len(strResponse)

	}

	strIP := w.ip
	intLen := len(w.timeStr) + len(w.latencyStr) + len(strIP) + len(w.cmd) +
		intRequestLen + intResponseLen + intErrorLen + 100
	if intLen > sb.Cap() {
		sb.Grow(sb.Len() + intLen)
	}

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
			if w.code != 200 || w.stacktraceBuffer.Len() > 0 {
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
		case "reqid":
			if w.reqid != _uuidDefault {
				sb.WriteString(w.reqid.String())
			}
		case "uid":
			if w.uid != _uuidDefault {
				sb.WriteString(w.uid.String())
			}
		case "rqqs":
			sb.WriteString(w.rqqs)
		case "useragent":
			if len(w.useragent_cmd) > 0 {
				if w.cmd == w.useragent_cmd {
					sb.WriteString(w.useragent)
				}
			} else {
				sb.WriteString(w.useragent)
			}
		case "rq":
			sb.WriteString("rq:")
			sb.WriteString(strReq)
		case "rs":
			sb.WriteString("rs:")
			if intErrorLen > 0 {
				sb.WriteString("error:")
				sb.WriteString(strErr)
				sb.WriteString("response: \n")
			}
			sb.WriteString(strResponse)
		}
		sb.WriteString(_tagDelimiters[i])
	}

	res := sb.String()
	sb.Reset()
	logTxt(&res)
	res = ""
}

// has application error with http code over 500 and hier
func (w *Logger) HasSystemError() bool { return w.stacktraceBuffer.Len() > 0 }

// application error with http code over 500 and hier
func (w *Logger) SystemError() string { return w.stacktraceBuffer.String() }

// everything that user will get (accumulation)
func (w *Logger) AddResponse(val string) {

	sz := len(val)
	if sz == 0 {
		return
	}

	buf := w.responseBuffer
	remains := buf.Cap() - buf.Len()
	if sz > remains {
		buf.Grow(sz + buf.Len())
	}

	buf.WriteString(val)
}
func (w *Logger) ClearResponse() {
	w.responseBuffer.Reset()
}

// for the log only, not for the client
func (w *Logger) AddStacktrace(errmsg string) *Logger {

	if len(errmsg) == 0 {
		return w
	}
	buf := w.stacktraceBuffer

	buf.WriteString("ERROR:")
	buf.WriteString(errmsg)
	buf.WriteString(NEW_LINE)
	if w.ResponseCode() == http.StatusOK {
		w.SetResponseCode(http.StatusInternalServerError)
	}
	printStackTrace(buf)
	buf.WriteString(NEW_LINE)
	return w
}

// for the log only, not for the client
func (w *Logger) RawData() string { return w.recRawBuffer.String() }

// recovers from a panic and logs the error if it exsits.
// usage: log.Recover(recover())
func (w *Logger) Recover(r interface{}) {

	if r == nil {
		return
	}
	msg := "panic: " + _recoverlog(r)
	w.AddStacktrace(msg)
}

// recovers from a panic, marks the panic place and logs the error if it exsits.
// usage: log.Recover("some place marker",recover())
func (w *Logger) RecoverWithMark(mark string, r interface{}) {

	if r == nil {
		return
	}
	msg := mark + " panic: " + _recoverlog(r)
	w.AddStacktrace(msg)
}

// recovers from a panic and returns an error if it exists
// usage: log.Recover("some place marker",recover())
func (w *Logger) RecoverNoLog(r interface{}) error {

	if r == nil {
		return nil
	}
	msg := "panic: " + _recoverlog(r)
	return errors.New(msg)

}
func _recoverlog(r interface{}) (msg string) {

	if r == nil {
		return ""
	}
	switch r.(type) {
	case string:
		msg = r.(string)
	case error:
		msg = r.(error).Error()
	default:
		msg = "unknown request fatal error"
	}
	fmt.Println(msg)
	return
}
