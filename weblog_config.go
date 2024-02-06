package weblog

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	MAX_CONFIG_STRING_SIZE int64 = 100000
)

var (
	_configFilePath           string
	_default_config_file_name        = `weblog.config`
	NewLine                   string = "\n"
	PathSeparator             string = string(os.PathSeparator)
)

// split config file on map[string]string
func configExtractProps(cfgstring string) (map[string]string, error) {

	intLen := len(cfgstring)
	if intLen == 0 {
		return nil, errors.New("validation error: config string is empty")
	}
	if intLen > int(MAX_CONFIG_STRING_SIZE) {
		return nil, errors.New("validation error: config file is too large (expected 100000): " + strconv.Itoa(intLen))
	}

	scanner := bufio.NewScanner(strings.NewReader(cfgstring))
	scanner.Split(bufio.ScanLines)

	props := make(map[string]string)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if len(line) > 0 && line[0] == '#' {
			continue
		}
		if len(line) == 0 || !strings.Contains(line, "=") {
			continue
		}
		arr := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(arr[0])
		val := strings.TrimSpace(arr[1])
		valtmp, _ := strconv.Unquote(val)
		if len(valtmp) > 0 {
			val = normalizeNewlines(valtmp)
		}
		props[key] = val
		// val := props[strings.TrimSpace(arr[0])]
		// val = strconv.Quote(val)
		// fmt.Println(runtime.GOOS)

	}

	if scanner.Err() != nil {
		return nil, errors.New("reading config: " + scanner.Err().Error())
	}

	return props, nil
}

// reads configuration file
func configReadFile(configFilePath string) (val string, err error) {

	var (
		file *os.File
		fi   fs.FileInfo
	)

	configFilePath = strings.TrimSpace(configFilePath)
	if len(configFilePath) > 0 {
		err = validatePath(configFilePath, false)
		if err != nil {
			return
		}
		if !strings.HasSuffix(configFilePath, PathSeparator) {
			configFilePath = configFilePath + PathSeparator
		}
	}

	if len(configFilePath) > 0 {
		_configFilePath = configFilePath + _default_config_file_name

	} else {
		_configFilePath = getExePath() + _default_config_file_name
	}
	if _, err = os.Stat(_configFilePath); err != nil {
		return "", err
	}

	fmt.Println("config file location: " + _configFilePath)

	file, err = os.Open(_configFilePath)
	defer func() {
		if err = file.Close(); err != nil {
			err = errors.New("on config file closing: " + err.Error())
		}
	}()

	if err != nil {
		return
	}
	fi, err = file.Stat()
	if err != nil {
		err = errors.New("config file get Stat() error: " + err.Error())
		return
	}

	fsize := fi.Size()
	if fsize > MAX_CONFIG_STRING_SIZE {
		err = errors.New("config file is too large (expected 100000): " + strconv.FormatInt(fsize, 10))
	}

	sb := new(strings.Builder)
	sb.Grow(int(fsize))
	io.Copy(sb, file)

	val = sb.String()
	sb.Reset()

	return val, err
}

func configValidateAndAssignProps(props map[string]string) (err error) {

	var (
		intTmp int
	)

	strTmp := props["log_path"]
	if len(strTmp) == 0 {
		if len(_logPath) == 0 {
			strTmp = getExePath()
		} else {
			strTmp = _logPath
		}
	}
	err = validatePath(strTmp, true)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(strTmp, PathSeparator) {
		strTmp = strTmp + PathSeparator
	}
	_logPath = strTmp

	strTmp = strings.TrimSpace(props["is_standalone"])
	if len(strTmp) > 0 {
		val, err := strconv.ParseBool(strTmp)
		if err == nil {
			_isStandalone = val
		} else {
			_isStandalone = true
			return errors.New("config: is_standalone boolean expected: " + err.Error())
		}
	}

	strTmp = strings.TrimSpace(props["service_abbr"])
	if len(strTmp) > 0 {
		if _rxSrvAbbr.MatchString(strTmp) {
			_serviceAbbr = strTmp
		} else {
			if !_isStandalone {
				printError("service name should by 5 letters in caps. F.e.: LOGER")
				os.Exit(1)
			}
		}
	}

	strTmp = props["log_file_name_format"]
	if len(strTmp) > 0 {
		setFileNameFormat(strTmp)
	}

	intTmp, err = parseIntParam("log_max_files_count", props["log_max_files_count"],
		-1, 1000)
	if err != nil {
		return err
	}

	if intTmp <= 0 {
		_logFileCountLimit = -1
	} else {
		_logFileCountLimit = intTmp
	}

	// "info" - just query string (default) end response HTTP code
	// "trace" - as "debug" but request body and response body are cut up to 1KB
	// "debug" - full info: query string, request body, response body, (headers?), errors
	// "error" - full info only if error ocuures
	strTmp = props["log_level"]
	if len(strTmp) > 0 {
		strTmp = strings.TrimSpace(strTmp)
		if len(strTmp) > 0 {
			switch strings.ToLower(strTmp) {
			case "info":
				_loglevel = LOG_INFO
			case "trace":
				_loglevel = LOG_TRACE
			case "debug":
				_loglevel = LOG_DEBUG
			case "error":
				_loglevel = LOG_ERROR
			}
		}
	}

	// project name to cut stacktrace on errors
	strTmp = props["log_mark"]
	strTmp = strings.TrimSpace(strTmp)
	if len(strTmp) > 0 {
		_log_mark = strings.TrimSpace(strTmp)
	}

	strTmp = props["server_url"]
	strTmp = strings.TrimSpace(strTmp)
	if len(strTmp) > 0 {
		setLogServerURL(strTmp)
	}

	strTmp = props["log_file_header_format"]
	if len(strTmp) > 0 {
		err = setLogFileFormat(strTmp)
	} else {
		err = setLogFileFormat(_log_format)
	}
	if err != nil {
		fmt.Printf("can not parse log file header format: %s\n", err.Error())
		os.Exit(1)
	}

	return nil
}

func initialize_config(configPath string) (err error) {

	defer func() {
		_recover(recover(), true)
	}()

	strConfig, err := configReadFile(configPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// default faluess
		err = nil
		return
	}

	props, err := configExtractProps(strConfig)
	if err != nil {
		return err
	}
	if len(props) == 0 {
		return err
	}

	return configValidateAndAssignProps(props)
}

func parseIntParam(key string, val string, min int, max int) (int, error) {

	if len(val) > 0 {
		intTmp, err := strconv.Atoi(val)
		if err != nil {
			return -1, errors.New("config: " + key + " must be integer value")
		} else {
			if intTmp < min || max <= intTmp {
				return -1, errors.New("config: " + key + " must be integer within " +
					strconv.Itoa(min) + ".." + strconv.Itoa(max) + " interval")
			} else {
				return intTmp, nil
			}
		}
	}
	return -1, nil
}

// NormalizeNewlines normalizes \r\n (windows) and \r (mac) into \n (unix)
func normalizeNewlines(val string) string {
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

func getExePath() string {

	ex, err := os.Executable()
	if err != nil {
		fmt.Print("can not get current path: ", NewLine, err.Error())
		os.Exit(1)
	}

	return filepath.Dir(ex) + string(os.PathSeparator)
}
