package rolling

import (
	"errors"
	"fmt"
	"github.com/robfig/cron"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RollingPolicies give out 3 policy for rolling.
const (
	WithoutRolling = iota
	TimeRolling
	VolumeRolling
)

const (
	rollingTimePattern = "*/10 * * * * ?"
	backupTimeFormat   = "2006-01-02T15-04-05.000"
	compressSuffix     = ".gz"
	defaultMaxSize     = 100
)

var _ io.WriteCloser = (*Logger)(nil)

var (
	// currentTime exists, so it can be mocked out by tests.
	currentTime = time.Now

	// DefaultFileMode set the default open mode rw-r--r-- by default
	DefaultFileMode = os.FileMode(0644)
	// DefaultFileFlag set the default file flag
	DefaultFileFlag = os.O_RDWR | os.O_CREATE | os.O_APPEND

	// megabyte is the conversion factor between MaxSize and bytes.  It is a
	// variable so tests can mock it out and not need to write megabytes of data
	// to disk.
	megabyte = 1024 * 1024
)

type Logger struct {
	LogPath  string `json:"logPath" yaml:"logPath"`
	Filename string `json:"filename" yaml:"filename"`

	// MaxAge is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename.  Note that a day is defined as 24
	// hours and may not exactly correspond to calendar days due to daylight
	// savings, leap seconds, etc. The default is not to remove old log files
	// based on age.
	MaxAge int `json:"maxAge" yaml:"maxAge"`
	// MaxRemain will auto clear the rolling file list, set 0 will disable auto clean
	MaxRemain int `json:"max_remain"`

	// RollingPolicy give out the rolling policy
	// We got 3 policies(actually, 2):
	//
	//	1. WithoutRolling: no rolling will happen
	//	2. TimeRolling: rolling by time
	//	3. VolumeRolling: rolling by file size
	RollingPolicy int `json:"rolling_policy"`
	MaxSize       int `json:"max_size"`

	// Compress will compress log file with gzip
	Compress bool `json:"compress"`

	// LocalTime determines if the time used for formatting the timestamps in
	// backup files is the computer's local time.  The default is to use UTC
	// time.
	LocalTime bool `json:"localtime" yaml:"localtime"`

	file      *os.File
	mu        sync.Mutex
	lock      sync.Mutex
	absPath   string
	fire      chan string
	startAt   time.Time
	cr        *cron.Cron
	millCh    chan bool
	startMill sync.Once
}

func defaultLogWriter() *Logger {
	return &Logger{
		LogPath:       os.TempDir(),
		Filename:      "all.log",
		MaxAge:        30,
		MaxRemain:     30,
		RollingPolicy: VolumeRolling,
		MaxSize:       15,
		Compress:      false,
		LocalTime:     false,
		fire:          make(chan string),
		startAt:       time.Now(),
		cr:            cron.New(),
	}
}

func NewWriter(options ...Option) (*Logger, error) {
	logger := defaultLogWriter()
	for _, opt := range options {
		opt(logger)
	}

	// make dir for path if not exist
	if err := os.MkdirAll(logger.LogPath, 0744); err != nil {
		return nil, err
	}

	fp := path.Join(logger.LogPath, logger.Filename)
	file, err := os.OpenFile(fp, DefaultFileFlag, DefaultFileMode)
	if err != nil {
		return nil, err
	}

	logger.file = file
	logger.absPath = fp

	switch logger.RollingPolicy {
	default:
		fallthrough
	case WithoutRolling:
		return logger, nil
	case TimeRolling:
		if err := logger.cr.AddFunc(rollingTimePattern, func() {
			logger.fire <- logger.backupName(logger.LogPath, logger.Filename, logger.LocalTime)
		}); err != nil {
			return nil, err
		}
		logger.cr.Start()
	}

	return logger, nil
}

func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	writeLen := int64(len(p))
	if writeLen > l.max() {
		return 0, fmt.Errorf(
			"write length %d exceeds maximum file size %d", writeLen, l.max(),
		)
	}

	if l.RollingPolicy == TimeRolling {
		select {
		case <-l.fire:
			if err := l.rotate(); err != nil {
				return 0, err
			}
		default:
			// 防止每天产生的日志文件过大
			if info, err := l.file.Stat(); err == nil && info.Size()+writeLen > l.max() {
				if err := l.rotate(); err != nil {
					return 0, err
				}
			}
		}
	} else if l.RollingPolicy == VolumeRolling {
		if info, err := l.file.Stat(); err == nil && info.Size()+writeLen > l.max() {
			if err := l.rotate(); err != nil {
				return 0, err
			}
		}
	}

	n, err = l.file.Write(p)
	return
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.close()
}

func (l *Logger) rotate() error {
	if err := l.close(); err != nil {
		return err
	}
	if err := l.openNew(); err != nil {
		return err
	}
	l.mill()
	return nil
}

// close the file if it is open.
func (l *Logger) close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// openNew opens a new log file for writing, moving any old log file out of the
// way.  This method assume the file has already been closed.
func (l *Logger) openNew() error {
	err := os.MkdirAll(l.LogPath, 0744)
	if err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}
	name := l.absPath
	mode := os.FileMode(0644)
	info, err := os.Stat(name)
	if err == nil {
		mode = info.Mode()

		newName := l.backupName(l.LogPath, l.Filename, l.LocalTime)
		if err := os.Rename(name, newName); err != nil {
			return fmt.Errorf("can't rename log file: %s", err)
		}
	}

	// we use truncate here because this should only get called when we've moved
	// the file ourselves. if someone else creates the file in the meantime,
	// just wipe out the contents.
	f, err := os.OpenFile(name, DefaultFileFlag, mode)
	if err != nil {
		return fmt.Errorf("can't open new logfile: %s", err)
	}
	l.file = f

	return nil
}

func (l *Logger) mill() {
	l.startMill.Do(func() {
		l.millCh = make(chan bool, 1)
		go l.millRun()
	})
	select {
	case l.millCh <- true:
	default:
	}
}

// millRun runs in a goroutine to manage post-rotation compression and removal
// of old log files.
func (l *Logger) millRun() {
	for range l.millCh {
		_ = l.millRunOnce()
	}
}

// millRunOnce performs compression and removal of stale log files.
// Log files are compressed if enabled via configuration and old log
// files are removed, keeping at most l.MaxBackups files, as long as
// none of them are older than MaxAge.
func (l *Logger) millRunOnce() error {
	if l.MaxRemain == 0 && l.MaxAge == 0 && !l.Compress {
		return nil
	}

	files, err := l.oldLogFiles()
	if err != nil {
		return err
	}

	var remove []logInfo

	if l.MaxRemain > 0 && l.MaxRemain < len(files) {
		preserved := make(map[string]bool)
		var remaining []logInfo
		for _, f := range files {
			fn := f.Name()
			if strings.HasSuffix(fn, compressSuffix) {
				fn = fn[:len(fn)-len(compressSuffix)]
			}
			preserved[fn] = true

			if len(preserved) > l.MaxRemain {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}

	if l.MaxAge > 0 {
		diff := time.Duration(int64(24*time.Hour) * int64(l.MaxAge))
		cutoff := currentTime().Add(-1 * diff)

		var remaining []logInfo
		for _, f := range files {
			if f.timestamp.Before(cutoff) {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}

	for _, f := range remove {
		errRemove := os.Remove(filepath.Join(l.LogPath, f.Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}

	return err
}

// oldLogFiles returns the list of backup log files stored in the same
// directory as the current log file, sorted by ModTime
func (l *Logger) oldLogFiles() ([]logInfo, error) {
	files, err := ioutil.ReadDir(l.LogPath)
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %s", err)
	}
	var logFiles []logInfo

	prefix, ext := l.prefixAndExt()

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if t, err := l.timeFromName(f.Name(), prefix, ext); err == nil {
			logFiles = append(logFiles, logInfo{t, f})
			continue
		}
		if t, err := l.timeFromName(f.Name(), prefix, ext+compressSuffix); err == nil {
			logFiles = append(logFiles, logInfo{t, f})
			continue
		}
	}
	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// prefixAndExt returns the filename part and extension part from the Logger's
// filename.
func (l *Logger) prefixAndExt() (prefix, ext string) {
	filename := l.Filename
	ext = filepath.Ext(l.Filename)
	prefix = filename[:len(filename)-len(ext)] + "-"
	return prefix, ext
}

// timeFromName extracts the formatted time from the filename by stripping off
// the prefix and extension. This prevents someone's filename from confusing time.parse.
func (l *Logger) timeFromName(filename, prefix, ext string) (time.Time, error) {
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, errors.New("mismatched prefix")
	}
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, errors.New("mismatched extension")
	}
	ts := filename[len(prefix) : len(filename)-len(ext)]
	return time.Parse(backupTimeFormat, ts)
}

// max returns the maximum size in bytes of log files before rolling.
func (l *Logger) max() int64 {
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(l.MaxSize) * int64(megabyte)
}

// backupName creates a new filename from the given name, inserting a timestamp
// between the filename and the extension, using the local time if requested
// (otherwise UTC).
func (l *Logger) backupName(dir, filename string, local bool) string {
	l.lock.Lock()
	defer l.lock.Unlock()
	ext := filepath.Ext(filename)
	prefix := filename[:len(filename)-len(ext)]
	t := currentTime()
	if !local {
		t = t.UTC()
	}

	l.startAt = time.Now()

	timestamp := t.Format(backupTimeFormat)
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", prefix, timestamp, ext))
}

// logInfo is a convenience struct to return the filename and its embedded
// timestamp.
type logInfo struct {
	timestamp time.Time
	os.FileInfo
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []logInfo

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}
