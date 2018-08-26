package log4go

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type jsonProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type jsonFilter struct {
	Enabled  string         `json:"enable"`
	Tag      string         `json:"tag"`
	Type     string         `json:"type"`
	Level    string         `json:"level"`
	Property []jsonProperty `json:"property"`
}

type jsonLoggerConfig struct {
	Filter []jsonFilter `json:"filters"`
}

// Load JSON configuration; see example.json for documentation
func (log Logger) LoadJsonConfiguration(filename string) {
	log.Close()

	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Could not read %q: %s\n", filename, err)
		os.Exit(1)
	}

	xc := new(jsonLoggerConfig)
	if err := json.Unmarshal(contents, xc); err != nil {
		fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Could not parse JSON configuration in %q: %s\n", filename, err)
		os.Exit(1)
	}

	for _, jsonfilt := range xc.Filter {
		var filt LogWriter
		var lvl level
		bad, good, enabled := false, true, false

		// Check required children
		if len(jsonfilt.Enabled) == 0 {
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required child <%s> for filter missing in %s\n", "filter", filename)
			bad = true
		} else {
			enabled = jsonfilt.Enabled != "false"
		}
		if len(jsonfilt.Tag) == 0 {
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required child <%s> for filter missing in %s\n", "tag", filename)
			bad = true
		}
		if len(jsonfilt.Type) == 0 {
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required child <%s> for filter missing in %s\n", "type", filename)
			bad = true
		}
		if len(jsonfilt.Level) == 0 {
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required child <%s> for filter missing in %s\n", "level", filename)
			bad = true
		}

		switch jsonfilt.Level {
		case "FINEST":
			lvl = FINEST
		case "FINE":
			lvl = FINE
		case "DEBUG":
			lvl = DEBUG
		case "TRACE":
			lvl = TRACE
		case "INFO":
			lvl = INFO
		case "WARNING":
			lvl = WARNING
		case "ERROR":
			lvl = ERROR
		case "CRITICAL":
			lvl = CRITICAL
		default:
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required child <%s> for filter has unknown value in %s: %s\n", "level", filename, jsonfilt.Level)
			bad = true
		}

		// Just so all of the required attributes are errored at the same time if missing
		if bad {
			os.Exit(1)
		}

		switch jsonfilt.Type {
		case "console":
			filt, good = jsonToConsoleLogWriter(filename, jsonfilt.Property, enabled)
		case "file":
			filt, good = jsonToFileLogWriter(filename, jsonfilt.Property, enabled)
		case "xml":
			filt, good = jsonToXMLLogWriter(filename, jsonfilt.Property, enabled)
		case "socket":
			filt, good = jsonToSocketLogWriter(filename, jsonfilt.Property, enabled)
		default:
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Could not load JSON configuration in %s: unknown filter type \"%s\"\n", filename, jsonfilt.Type)
			os.Exit(1)
		}

		// Just so all of the required params are errored at the same time if wrong
		if !good {
			os.Exit(1)
		}

		// If we're disabled (syntax and correctness checks only), don't add to logger
		if !enabled {
			continue
		}

		log[jsonfilt.Tag] = &Filter{lvl, filt}
	}
}

func jsonToConsoleLogWriter(filename string, props []jsonProperty, enabled bool) (ConsoleLogWriter, bool) {
	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		default:
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Warning: Unknown property \"%s\" for console filter in %s\n", prop.Name, filename)
		}
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, true
	}

	return NewConsoleLogWriter(), true
}

// Parse a number with K/M/G suffixes based on thousands (1000) or 2^10 (1024)
/*func strToNumSuffix(str string, mult int) int {
	num := 1
	if len(str) > 1 {
		switch str[len(str)-1] {
		case 'G', 'g':
			num *= mult
			fallthrough
		case 'M', 'm':
			num *= mult
			fallthrough
		case 'K', 'k':
			num *= mult
			str = str[0 : len(str)-1]
		}
	}
	parsed, _ := strconv.Atoi(str)
	return parsed * num
}*/
func jsonToFileLogWriter(filename string, props []jsonProperty, enabled bool) (*FileLogWriter, bool) {
	logdir := ""
	file := ""
	filesuffix := ""
	format := "[%D %T] [%L] (%S) %M"
	maxlines := 0
	maxsize := 0
	daily := false
	rotate := false

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "logdir":
			logdir = strings.Trim(prop.Value, " \r\n")
			if os.MkdirAll(logdir, os.ModePerm) == nil {
				os.Chmod(logdir, os.ModePerm)
			}
		case "filename":
			file = strings.Trim(prop.Value, " \r\n")
		case "filesuffix":
			filesuffix = strings.Trim(prop.Value, " \r\n")
		case "format":
			format = strings.Trim(prop.Value, " \r\n")
		case "maxlines":
			maxlines = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1000)
		case "maxsize":
			maxsize = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1024)
		case "daily":
			daily = strings.Trim(prop.Value, " \r\n") != "false"
		case "rotate":
			rotate = strings.Trim(prop.Value, " \r\n") != "false"
		default:
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Warning: Unknown property \"%s\" for file filter in %s\n", prop.Name, filename)
		}
	}

	// Check properties
	if len(file) == 0 {
		fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required property \"%s\" for file filter missing in %s\n", "filename", filename)
		return nil, false
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, true
	}

	logpath := logdir + "/" + file
	fmt.Fprintf(os.Stdout, "logdir:%s, file:%s, filesuffix:%s, logpath:%s\n", logdir, file, filesuffix, logpath)
	flw := NewFileLogWriter(logpath, filesuffix, rotate)
	if flw == nil {
		return flw, false
	}
	flw.SetFormat(format)
	flw.SetRotateLines(maxlines)
	flw.SetRotateSize(maxsize)
	flw.SetRotateDaily(daily)
	return flw, true
}

func jsonToXMLLogWriter(filename string, props []jsonProperty, enabled bool) (*FileLogWriter, bool) {
	file := ""
	maxrecords := 0
	maxsize := 0
	daily := false
	rotate := false

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "filename":
			file = strings.Trim(prop.Value, " \r\n")
		case "maxrecords":
			maxrecords = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1000)
		case "maxsize":
			maxsize = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1024)
		case "daily":
			daily = strings.Trim(prop.Value, " \r\n") != "false"
		case "rotate":
			rotate = strings.Trim(prop.Value, " \r\n") != "false"
		default:
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Warning: Unknown property \"%s\" for json filter in %s\n", prop.Name, filename)
		}
	}

	// Check properties
	if len(file) == 0 {
		fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required property \"%s\" for json filter missing in %s\n", "filename", filename)
		return nil, false
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, true
	}

	xlw := NewXMLLogWriter(file, rotate)
	xlw.SetRotateLines(maxrecords)
	xlw.SetRotateSize(maxsize)
	xlw.SetRotateDaily(daily)
	return xlw, true
}

func jsonToSocketLogWriter(filename string, props []jsonProperty, enabled bool) (SocketLogWriter, bool) {
	endpoint := ""
	protocol := "udp"

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "endpoint":
			endpoint = strings.Trim(prop.Value, " \r\n")
		case "protocol":
			protocol = strings.Trim(prop.Value, " \r\n")
		default:
			fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Warning: Unknown property \"%s\" for file filter in %s\n", prop.Name, filename)
		}
	}

	// Check properties
	if len(endpoint) == 0 {
		fmt.Fprintf(os.Stderr, "LoadJsonConfiguration: Error: Required property \"%s\" for file filter missing in %s\n", "endpoint", filename)
		return nil, false
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, true
	}

	return NewSocketLogWriter(protocol, endpoint), true
}
