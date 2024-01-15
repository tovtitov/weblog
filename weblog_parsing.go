package weblog

import (
	"bytes"
	"errors"
	"strings"
)

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
		case "reqid":
			rec.SetRequestIdStr(vals[i])
		case "uid":
			rec.SetUserIdStr(vals[i])
		case "rqqs":
			rec.SetRequestQS(vals[i])
		case "rq":
			rec.SetRequest([]byte(vals[i]))
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
		buf := rec.recRawBuffer
		diff := buf.Cap() - buf.Len()
		if intLen > diff {
			buf.Grow(intLen + buf.Len() + len(vals)*2)
		}
		for _, v := range vals {
			buf.WriteString(v)
			buf.WriteString(NEW_LINE)
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

	if oLog.HasSystemError() {
		if returnObjectOnError {
			return oLog, errors.New(oLog.SystemError())
		}
		return nil, errors.New(oLog.SystemError())
	}
	return oLog, nil

}
