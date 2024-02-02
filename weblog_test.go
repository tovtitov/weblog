package weblog

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// /Users/titov/Code/NewSecretProject/docs/1.2_Logging.txt

func TestWriteCycle(t *testing.T) {

	// "/Users/titov/Code/NewSecretProject/weblog/"
	Init()
	SetFileNameFormat("D")
	for i := 0; i < 10; i++ {

		msg := "very long test data " + strconv.Itoa(i)
		LogTxtTest(&msg)

	}
	Close()
}

func TestWriteRequest(t *testing.T) {

	// "/Users/titov/Code/NewSecretProject/weblog/"
	// InitF(
	// 	"$$$\tdatetime\terr\tcmd\tcode\tlatency\tip\tsrvc\trqct\trsct\treqid\tuid\trqqs\tuseragent\r\nrq\r\nrs")
	SetLogFileCountLimit(10)
	Init()
	SetLogLevel("info")
	SetLogPath("/Users/titov/Code/Licence")
	Reconfigure()
	log := NewLogger()

	start := time.Now()

	// time.Sleep(12 * time.Millisecond)

	elapsed := int64(time.Since(start)) / int64(time.Millisecond)
	log.SetLatency(elapsed)

	log.SetCommand("object.action")
	log.SetIP("203.0.113.195, 70.41.3.18, 150.172.238.178, 203.0.113.195, 70.41.3.18, 150.172.238.178, 203.0.113.195, 70.41.3.18, 150.172.238.178, 203.0.113.195, 70.41.3.18, 150.172.238.178")
	log.SetUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	log.SetUserAgentCommand("obj.action")
	fmt.Println(log.ip)
	log.SetRequest([]byte(`some 
	multiline 
	request`))
	log.SetResponseBinary([]byte(`some 
	multiline 
	response`))
	log.SetRequestQS("qqq=wwww&eee=rrr")
	// log.SetIsRequestBinary(true)
	log.SetResponseCode(200)
	log.SetRequestContentType("text/plain")
	log.SetResponseContentType("application/json")

	log.WriteRequest()
	log.Reset()
	Close()

}

func TestSendMessage(t *testing.T) {

	Init()

	arr := []byte(_log_format)
	_, code, err := sendMessage(SERVER_URL_LOG_RECORD, &arr)
	if code == 200 && err == nil {
		// meesage was sent to server
		if _fileLog != nil {
			// no need in log file
			_muLogWrite.Lock()
			_fileLog.Close()
			_muLogWrite.Unlock()
			// go sendBatch(_fileLog.Name())
			_fileLog = nil

		}
		return
	} else {
		assert.NoError(t, err, "TestSendMessage: log record not sent")
	}
	Close()
}

func TestParseLogRecOnServerside(t *testing.T) {
	val := `^^^ datetime err cmd code latency ip rqct rsct reqid uid rqqs
rq
rs

^^^	2022-06-08 10:41:08.897867		object.action	200	12	127.0.0.1	text/plain	application/json			qqq=wwww&eee=rrr
rq:some 
	multiline 
	request
rs:some 
	multiline 
	response`
	//"/Users/titov/Code/NewSecretProject/weblog/",
	Init()
	oLog, err := ParseLogRecordString(&val, false)
	if err != nil {
		require.NoError(t, err, err.Error())
	}
	_ = oLog

}
func TestParseLogFormat(t *testing.T) {
	parseLogFileHeader(_log_format)
}

func TestFileWrite(t *testing.T) {

	fileLog, e := os.OpenFile("logfile", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if e != nil {
		e := "can not open log file: " + e.Error()
		fmt.Println(e)
	}
	fileLog.WriteString("1 qwertyuiouytryewtyuiytr\n")

	fileLog.WriteString("2 qwertyuiouytryewtyuiytr\n")
	fileLog.Close()
}

func TestQSParsing(t *testing.T) {
	var (
		arr   []string
		pair  []string
		mapQS map[string]string
	)
	qs := "qqq=www"
	if len(qs) == 0 {
		return
	}
	arr = strings.Split(qs, "&")
	if len(arr) > 0 {
		mapQS = make(map[string]string)
		for _, kv := range arr {
			pair = strings.Split(kv, "=")
			if len(pair) == 2 {
				mapQS[pair[0]] = pair[1]
			}
		}
	}
	res := mapQS["qqq"]
	_ = res
	return
}
