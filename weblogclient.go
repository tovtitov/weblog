package weblog

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	SERVER_URL_PING = "http://localhost:5678/ping.send"
)

func initializeClient() error {

	SERVER_URL_LOG_RECORD = SERVER_URL_LOG_RECORD + _serviceAbbr
	SERVER_URL_LOG_BATCH = SERVER_URL_LOG_BATCH + _serviceAbbr

	transportPointer, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return errors.New("DefaultTransport not an *http.Transport")
	}
	transport := *transportPointer // dereference it to get a copy of the struct that the pointer points to
	transport.MaxConnsPerHost = 100
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 100
	transport.IdleConnTimeout = 90 * time.Second

	client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &transport,
	}

	// scan weblogsrv availability (if not run on WebLogSrv)
	if !_isStandalone {
		// go func() {
		for _isAppLive {

			resp, err := client.Get(SERVER_URL_PING)
			if err != nil {
				_isWebLogSrvAvailable = false
			} else {
				if resp != nil {
					if resp.StatusCode == 200 {
						_isWebLogSrvAvailable = true
					} else {
						_isWebLogSrvAvailable = false
					}
					if resp.Body != nil {
						resp.Body.Close()
					}
				}
			}
			time.Sleep(SLEEP_PING_PERIOD_SEC * time.Second)
		}
		// }()
	}

	return nil
}

// Sends recently closed file to WebLogSrv after successfull connection
// On error, file will stay in log folder. WebLogSrv should load it the next day
// func sendBatch(logFileName string) {

// 	if len(logFileName) > 0 && _isWebLogSrvAvailable && len(logFileName) > 0 {

// 		// send recently closed log-file to WebLogSrv
// 		arr, err := ioutil.ReadFile(logFileName)
// 		if err == nil && arr != nil && len(arr) > 0 {
// 			_, code, err := sendMessage(SERVER_URL+SERVER_URL_LOG_BATCH, &arr)
// 			if code == 200 && err == nil {
// 				os.Remove(logFileName)
// 			}
// 		}
// 	}
// }

func sendMessage(url string, msg *[]byte) (string, int, error) {

	defer func() {
		recoverTaskAndWork(recover())
	}()
	var (
		resp *http.Response
		err  error
	)

	req, err := http.NewRequest("POST", url, bytes.NewReader(*msg))
	req.Header.Set("Content-type", "text/plain")
	if err != nil {
		_isWebLogSrvAvailable = false
		return "", 500, err
	}

	client := http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err = client.Do(req)
	if err != nil {
		_isWebLogSrvAvailable = false
		if resp == nil {
			return "", http.StatusServiceUnavailable, err

		}
		return "", resp.StatusCode, err
	}

	// https://daryl-ng.medium.com/why-you-should-avoid-ioutil-readall-in-go-e6be4de180f8
	buf := new(strings.Builder)
	io.Copy(buf, resp.Body)
	resp.Body.Close()
	body_result := buf.String()
	buf.Reset()
	return body_result, resp.StatusCode, nil
}
