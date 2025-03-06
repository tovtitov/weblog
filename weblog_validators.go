package weblog

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func setLogServerURL(url string) bool {
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

// A - abbreviation, D - date, T - time, U - instance ID (UUID)
// f.e.:ADTU - SRV01.2022-12-12.15-03-12.b461cc28-8bab-4c19-8e25-f4c17faf5638.log
// by default: 2022-12-12.log
func setFileNameFormat(fileNameFormat string) bool {
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

	// createLogFileAgain()

	return true
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

func validatePath(path string, withWrite bool) (err error) {
	if len(path) == 0 {
		return
	}
	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return err
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return errors.New("config: invalid path: " + err.Error())
	} else {
		if withWrite {
			f, err := ioutil.TempFile(path, "tmp")
			if err != nil {
				return errors.New("config: can not write test file in " + path + ": " + err.Error())
			}
			f.Close()
			os.Remove(f.Name())
		}
	}
	return
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
