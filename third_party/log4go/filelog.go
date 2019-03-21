// Copyright (C) 2010, Kyle Lemons <kyle@kylelemons.net>.  All rights reserved.

package log4go

import (
	"fmt"
	"os"
	"time"
)

// This log writer sends output to a file
type FileLogWriter struct {
	rec chan *LogRecord
	rot chan bool

	// The opened file
	filename    string
	conf_fname  string
	filesuffix  string
	file        *os.File
	is_full     bool
	close_state int32

	// The logging format
	format string

	// File header/trailer
	header, trailer string

	// Rotate at linecount
	maxlines          int
	maxlines_curlines int

	// Rotate at size
	maxsize         int
	maxsize_cursize int

	// Rotate daily
	daily          bool
	daily_opendate int

	// Keep old logfiles (.001, .002, etc)
	rotate bool

	// the number of log files
	num int32

	// year month day
	year, month, day int

	close_chan chan bool
}

// This is the FileLogWriter's output method
func (w *FileLogWriter) LogWrite(rec *LogRecord) {
	if w.is_full {
		return
	}
	if w.close_state > 0 {
		return
	}
	w.rec <- rec
}

func (w *FileLogWriter) Close() {
	w.close_state = 1
	w.close_chan <- true
	for {
		if w.close_state == 2 {
			break
		}
		time.Sleep(time.Millisecond * 1000)
	}
	//close(w.rec)
}

// NewFileLogWriter creates a new LogWriter which writes to the given file and
// has rotation enabled if rotate is true.
//
// If rotate is true, any time a new log file is opened, the old one is renamed
// with a .### extension to preserve it.  The various Set* methods can be used
// to configure log rotation based on lines, size, and daily.
//
// The standard log-line format is:
//   [%D %T] [%L] (%S) %M
func NewFileLogWriter(fname string, fsuffix string, rotate bool) *FileLogWriter {
	w := &FileLogWriter{
		rec:        make(chan *LogRecord, LogBufferLength),
		rot:        make(chan bool),
		filename:   fname,
		conf_fname: fname,
		filesuffix: fsuffix,
		format:     "[%D %T] [%L] (%S) %M",
		rotate:     rotate,
		num:        0,
		close_chan: make(chan bool),
	}

	w.CheckNumberStart()

	// open the file for the first time
	if err := w.intRotate(); err != nil {
		fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.filename, err)
		return nil
	}

	go func() {
		defer func() {
			if w.file != nil {
				fmt.Fprint(w.file, FormatLogRecord(w.trailer, &LogRecord{Created: time.Now()}))
				w.file.Close()
			}
		}()

		for {
			select {
			case <-w.rot:
				if err := w.intRotate(); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.filename, err)
					return
				}
			case rec, ok := <-w.rec:
				if !ok {
					w.is_full = true
					return
				}
				now := time.Now()
				if (w.maxlines > 0 && w.maxlines_curlines >= w.maxlines) ||
					(w.maxsize > 0 && w.maxsize_cursize >= w.maxsize) ||
					(w.daily && now.Day() != w.daily_opendate) {
					if err := w.intRotate(); err != nil {
						w.is_full = true
						fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.filename, err)
						return
					}
				}

				// Perform the write
				n, err := fmt.Fprint(w.file, FormatLogRecord(w.format, rec))
				if err != nil {
					w.is_full = true
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.filename, err)
					return
				}

				// Update the counts
				w.maxlines_curlines++
				w.maxsize_cursize += n
			case <-w.close_chan:
				close(w.rec)
				w.close_state = 2
				return
			}
		}
	}()

	return w
}

// check number start
func (w *FileLogWriter) CheckNumberStart() {
	// open file by date
	now_time := time.Now()
	y := now_time.Year()
	m := now_time.Month()
	d := now_time.Day()

	w.year = y
	w.month = int(m)
	w.day = d

	filename := fmt.Sprintf("%s_%d-%02d-%02d", w.conf_fname, y, m, d)

	var fname string
	num := int32(0)
	for {
		if num == 0 {
			fname = filename + fmt.Sprintf(".%s", w.filesuffix)
		} else {
			fname = filename + fmt.Sprintf("_%d.%s", num, w.filesuffix)
		}
		_, err := os.Lstat(fname)
		if err != nil {
			break
		}
		num += 1
	}

	w.num = num
}

// Request that the logs rotate
func (w *FileLogWriter) Rotate() {
	w.rot <- true
}

// If this is called in a threaded context, it MUST be synchronized
func (w *FileLogWriter) intRotate() error {
	// Close any log file that may be open
	if w.file != nil {
		fmt.Fprint(w.file, FormatLogRecord(w.trailer, &LogRecord{Created: time.Now()}))
		w.file.Close()
	}

	// open file by date
	now_time := time.Now()
	y := now_time.Year()
	m := now_time.Month()
	d := now_time.Day()

	if w.year != y || w.month != int(m) || w.day != d {
		w.year = y
		w.month = int(m)
		w.day = d
		w.num = 0
	}

	filename := fmt.Sprintf("%s_%d-%02d-%02d", w.conf_fname, w.year, w.month, w.day)

	// If we are keeping log files, move it to the next available number
	if w.rotate {
		if w.filename == w.conf_fname {
			w.filename = fmt.Sprintf("%s.%s", filename, w.filesuffix)
		}
		_, err := os.Lstat(w.filename)
		if err == nil { // file exists
			// Find the next available number
			//num := 1

			if w.num == 0 {
				w.filename = filename + fmt.Sprintf(".%s", w.filesuffix)
			} else {
				w.filename = filename + fmt.Sprintf("_%d.%s", w.num, w.filesuffix)
			}
			/*for ; err == nil && num <= 999; num++ {
				fname = filename + fmt.Sprintf("_%03d.%s", num, w.filesuffix)
				_, err = os.Lstat(fname)
			}*/
			// return error if the last file checked still existed
			//if err == nil {
			//	return fmt.Errorf("Rotate: Cannot find free log number to rename %s\n", filename)
			//}

			// Rename the file to its newfound home
			/*err = os.Rename(w.filename, fname)
			if err != nil {
				return fmt.Errorf("Rotate: %s\n", err)
			}*/
			w.num += 1
		}
	}

	// Open the log file
	fd, err := os.OpenFile(w.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	w.file = fd

	now := time.Now()
	fmt.Fprint(w.file, FormatLogRecord(w.header, &LogRecord{Created: now}))

	// Set the daily open date to the current date
	w.daily_opendate = now.Day()

	// initialize rotation values
	w.maxlines_curlines = 0
	w.maxsize_cursize = 0

	return nil
}

// 设置文件后缀名
func (w *FileLogWriter) SetSuffix(suffix string) *FileLogWriter {
	w.filesuffix = suffix
	return w
}

// Set the logging format (chainable).  Must be called before the first log
// message is written.
func (w *FileLogWriter) SetFormat(format string) *FileLogWriter {
	w.format = format
	return w
}

// Set the logfile header and footer (chainable).  Must be called before the first log
// message is written.  These are formatted similar to the FormatLogRecord (e.g.
// you can use %D and %T in your header/footer for date and time).
func (w *FileLogWriter) SetHeadFoot(head, foot string) *FileLogWriter {
	w.header, w.trailer = head, foot
	if w.maxlines_curlines == 0 {
		fmt.Fprint(w.file, FormatLogRecord(w.header, &LogRecord{Created: time.Now()}))
	}
	return w
}

// Set rotate at linecount (chainable). Must be called before the first log
// message is written.
func (w *FileLogWriter) SetRotateLines(maxlines int) *FileLogWriter {
	//fmt.Fprintf(os.Stderr, "FileLogWriter.SetRotateLines: %v\n", maxlines)
	w.maxlines = maxlines
	return w
}

// Set rotate at size (chainable). Must be called before the first log message
// is written.
func (w *FileLogWriter) SetRotateSize(maxsize int) *FileLogWriter {
	//fmt.Fprintf(os.Stderr, "FileLogWriter.SetRotateSize: %v\n", maxsize)
	w.maxsize = maxsize
	return w
}

// Set rotate daily (chainable). Must be called before the first log message is
// written.
func (w *FileLogWriter) SetRotateDaily(daily bool) *FileLogWriter {
	//fmt.Fprintf(os.Stderr, "FileLogWriter.SetRotateDaily: %v\n", daily)
	w.daily = daily
	return w
}

// SetRotate changes whether or not the old logs are kept. (chainable) Must be
// called before the first log message is written.  If rotate is false, the
// files are overwritten; otherwise, they are rotated to another file before the
// new log is opened.
func (w *FileLogWriter) SetRotate(rotate bool) *FileLogWriter {
	//fmt.Fprintf(os.Stderr, "FileLogWriter.SetRotate: %v\n", rotate)
	w.rotate = rotate
	return w
}

// NewXMLLogWriter is a utility method for creating a FileLogWriter set up to
// output XML record log messages instead of line-based ones.
func NewXMLLogWriter(fname string, rotate bool) *FileLogWriter {
	return NewFileLogWriter(fname, "", rotate).SetFormat(
		`	<record level="%L">
		<timestamp>%D %T</timestamp>
		<source>%S</source>
		<message>%M</message>
	</record>`).SetHeadFoot("<log created=\"%D %T\">", "</log>")
}
