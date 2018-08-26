package log

import (
	"fmt"
	"ih_server/third_party/log4go"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var logger log4go.Logger
var tracer log4go.Logger
var trace = false

func Init(log_config_file string, trace_config_file string, trace_enable bool) {
	if log_config_file != "" {
		logger = make(log4go.Logger)
		logger.LoadJsonConfiguration(log_config_file)
	}

	if trace_config_file != "" {
		tracer = make(log4go.Logger)
		tracer.LoadJsonConfiguration(trace_config_file)
	}

	trace = trace_enable
}

func Debug(arg0 interface{}, args ...interface{}) {
	if trace && tracer != nil {
		pc, _, line, ok := runtime.Caller(1)
		if ok {
			func_name := runtime.FuncForPC(pc).Name()
			switch arg0.(type) {
			case string:
				tracer.Debug("["+func_name+"]["+strconv.Itoa(line)+"]"+arg0.(string), args...)
			case func() string:
				tracer.Debug(arg0, args...)
			default:
				tracer.Debug("["+func_name+"]["+strconv.Itoa(line)+"]"+fmt.Sprint(arg0)+strings.Repeat(" %v", len(args)), args...)
			}

		} else {
			tracer.Debug(arg0, args...)
		}
	}
}

func Trace(arg0 interface{}, args ...interface{}) {
	if trace && tracer != nil {
		pc, _, line, ok := runtime.Caller(1)
		if ok {
			func_name := runtime.FuncForPC(pc).Name()
			switch arg0.(type) {
			case string:
				tracer.Trace("["+func_name+"]["+strconv.Itoa(line)+"]"+arg0.(string), args...)
			case func() string:
				tracer.Trace(arg0, args...)
			default:
				tracer.Trace("["+func_name+"]["+strconv.Itoa(line)+"]"+fmt.Sprint(arg0)+strings.Repeat(" %v", len(args)), args...)
			}

		} else {
			tracer.Trace(arg0, args...)
		}
	}
}

func Info(arg0 interface{}, args ...interface{}) (err error) {
	if logger != nil {
		logger.Info(arg0, args...)
	}
	if tracer != nil {
		tracer.Info(arg0, args...)
	}

	return
}

func Warn(arg0 interface{}, args ...interface{}) {
	if trace && tracer != nil {
		pc, _, line, ok := runtime.Caller(1)
		if ok {
			func_name := runtime.FuncForPC(pc).Name()
			switch arg0.(type) {
			case string:
				tracer.Warn("["+func_name+"]["+strconv.Itoa(line)+"]"+arg0.(string), args...)
			case func() string:
				tracer.Warn(arg0, args...)
			default:
				tracer.Warn("["+func_name+"]["+strconv.Itoa(line)+"]"+fmt.Sprint(arg0)+strings.Repeat(" %v", len(args)), args...)
			}

		} else {
			tracer.Warn(arg0, args...)
		}
	}
}

func Error(arg0 interface{}, args ...interface{}) (err error) {
	pc, _, line, ok := runtime.Caller(1)
	if ok {
		func_name := runtime.FuncForPC(pc).Name()
		switch arg0.(type) {
		case string:
			if logger != nil {
				logger.Error("["+func_name+"]["+strconv.Itoa(line)+"]"+arg0.(string), args...)
			}
			if tracer != nil {
				tracer.Error("["+func_name+"]["+strconv.Itoa(line)+"]"+arg0.(string), args...)
			}
		case func() string:
			if logger != nil {
				logger.Error(arg0, args...)
			}
			if tracer != nil {
				tracer.Error(arg0, args...)
			}
		default:
			if logger != nil {
				logger.Error("["+func_name+"]["+strconv.Itoa(line)+"]"+fmt.Sprint(arg0)+strings.Repeat(" %v", len(args)), args...)
			}
			if tracer != nil {
				tracer.Error("["+func_name+"]["+strconv.Itoa(line)+"]"+fmt.Sprint(arg0)+strings.Repeat(" %v", len(args)), args...)
			}
		}
	} else {
		if logger != nil {
			logger.Error(arg0, args...)
		}
		if tracer != nil {
			tracer.Error(arg0, args...)
		}
	}

	return
}

func Stack(err interface{}) {
	if err != nil {
		if logger != nil {
			logger.Error("<crash>%v %v", err, reflect.TypeOf(err))
		}
		if tracer != nil {
			tracer.Error("%v %v", err, reflect.TypeOf(err))
		}
	}

	for i := 0; i < 20; i++ {
		funcName, file, line, ok := runtime.Caller(i)
		if ok {
			if logger != nil {
				logger.Error("<stack>%v|%v|%v|%v}\n", i, runtime.FuncForPC(funcName).Name(), file, line)
			}
			if tracer != nil {
				tracer.Error("%v|%v|%v|%v}\n", i, runtime.FuncForPC(funcName).Name(), file, line)
			}
		}
	}
}

func Close() {
	if logger != nil {
		logger.Close()
	}

	if tracer != nil {
		tracer.Close()
	}
}

type DurationEvent struct {
	begin time.Time
}

func (this *DurationEvent) End(map[string]string) {

}

type Property struct {
	Name  string
	Value interface{}
}

func Event(name string, value interface{}, properties ...Property) {
	var s string
	if value == nil {
		s = fmt.Sprintf("<综合><" + name + ">")
	} else {
		s = fmt.Sprintf("<综合><"+name+" %v>", value)
	}
	if properties != nil {
		for _, v := range properties {
			s += fmt.Sprintf("<"+v.Name+" %v>", v.Value)
		}
	}

	if logger != nil {
		logger.Info(s)
	}
	if trace && tracer != nil {
		tracer.Info(s)
	}
}

func PlayerEvent(id int32, name string, value interface{}, properties ...Property) {
	var s string
	if value == nil {
		s = fmt.Sprintf("<玩家 %v><"+name+">", id)
	} else {
		s = fmt.Sprintf("<玩家 %v><"+name+" %v>", id, value)
	}
	if properties != nil {
		for _, v := range properties {
			s += fmt.Sprintf("<"+v.Name+" %v>", v.Value)
		}
	}

	if logger != nil {
		logger.Info(s)
	}
	if trace && tracer != nil {
		tracer.Info(s)
	}
}

func BeginEvent(name string, value interface{}) (e *DurationEvent) {
	e = &DurationEvent{time.Now()}

	return e
}
