package rolling

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// !!!NOTE!!!
//
// Running these tests in parallel will almost certainly cause sporadic (or even
// regular) failures, because they're all messing with the same global variable
// that controls the logic's mocked time.Now.  So... don't do that.

var fakeCurrentTime = time.Now()

func fakeTime() time.Time {
	return fakeCurrentTime
}

func TestNewFile(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestNewFile", t)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()

	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)
	existsWithContent(logFile(dir), b, t)
	fileCount(dir, 1, t)
}

func TestOpenExisting(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestOpenExisting", t)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	filename := logFile(dir)
	data := []byte("foo!")
	err := ioutil.WriteFile(filename, data, 0644)
	isNil(err, t)
	existsWithContent(filename, data, t)

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()))
	isNil(err, t)
	defer func(l *Logger) {
		err := l.Close()
		if err != nil {

		}
	}(l)
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	// make sure the file got appended
	existsWithContent(filename, append(data, b...), t)

	// make sure no other files were created
	fileCount(dir, 1, t)
}

func TestWriteTooLong(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestWriteTooLong", t)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()), WithMaxSize(5))
	isNil(err, t)
	defer func(l *Logger) {
		err := l.Close()
		if err != nil {

		}
	}(l)

	b := []byte("00000000000000000!!")
	n, err := l.Write(b)
	notNil(err, t)
	equals(0, n, t)
	equals(err.Error(),
		fmt.Sprintf("write length %d exceeds maximum file size %d", len(b), l.max()), t)
	_, err = os.Stat(logFile(dir))
	assert(os.IsNotExist(err), t, "File exists, but should not have been created")
}

func TestMakeLogDir(t *testing.T) {
	currentTime = fakeTime
	dir := time.Now().Format("TestMakeLogDir" + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()

	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)
	existsWithContent(logFile(dir), b, t)
	fileCount(dir, 1, t)
}

func TestDefaultFilename(t *testing.T) {
	currentTime = fakeTime
	dir := os.TempDir()
	filename := filepath.Join(dir, "all.log")
	defer func() {
		err := os.Remove(filename)
		if err != nil {
			return
		}
	}()

	l, err := NewWriter()
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()
	b := []byte("boo!")
	n, err := l.Write(b)

	isNil(err, t)
	equals(len(b), n, t)
	existsWithContent(filename, b, t)
}

func TestAutoRotate(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestAutoRotate", t)
	filename := logFile(dir)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()), WithMaxSize(10))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()

	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	b2 := []byte("0000000!")
	n, err = l.Write(b2)
	isNil(err, t)
	equals(len(b2), n, t)

	// the old logfile should be moved aside and the main logfile should have
	// only the last write-in it.
	existsWithContent(filename, b2, t)

	// the backup file will use the current fake time and have the old contents.
	existsWithContent(backupFile(dir), b, t)

	fileCount(dir, 2, t)
}

func TestFirstWriteRotate(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestFirstWriteRotate", t)
	filename := logFile(dir)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()), WithMaxSize(10))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()

	start := []byte("123456")
	err = ioutil.WriteFile(filename, start, 0600)
	isNil(err, t)

	newFakeTime()

	b := []byte("fo0o!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	existsWithContent(filename, b, t)
	existsWithContent(backupFile(dir), start, t)

	fileCount(dir, 2, t)
}

func TestMaxBackups(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestMaxBackups", t)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	filename := logFile(dir)
	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()), WithMaxSize(10), WithMaxRemain(1))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()

	b := []byte("boo")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	b2 := []byte("00000000")
	n, err = l.Write(b2)
	isNil(err, t)
	equals(len(b2), n, t)

	// this will use the new fake time
	secondFilename := backupFile(dir)
	existsWithContent(secondFilename, b, t)
	existsWithContent(filename, b2, t)

	fileCount(dir, 2, t)

	newFakeTime()
	b3 := []byte("111111")
	n, err = l.Write(b3)
	isNil(err, t)
	equals(len(b3), n, t)

	thirdFilename := backupFile(dir)
	existsWithContent(thirdFilename, b2, t)
	existsWithContent(filename, b3, t)

	<-time.After(time.Millisecond * 10)
	fileCount(dir, 2, t)

	// second file name should still exist
	existsWithContent(thirdFilename, b2, t)

	// should have deleted the first backup
	notExist(secondFilename, t)

	// now test that we don't delete directories or non-logfile files
	newFakeTime()

	notLogFile := logFile(dir) + ".foo"
	err = ioutil.WriteFile(notLogFile, []byte("data"), 0600)
	isNil(err, t)

	// Make a directory that exactly matches our log file filters... it still
	// shouldn't get caught by the deletion filter since it's a directory.
	notLogFileDir := backupFile(dir)
	err = os.Mkdir(notLogFileDir, 0700)
	isNil(err, t)

	newFakeTime()

	fourthFilename := backupFile(dir)
	compLogFile := fourthFilename + compressSuffix
	err = ioutil.WriteFile(compLogFile, []byte("compress"), 0644)
	isNil(err, t)

	b4 := []byte("22222222")
	n, err = l.Write(b4)
	isNil(err, t)
	equals(len(b4), n, t)

	existsWithContent(fourthFilename, b3, t)
	existsWithContent(fourthFilename+compressSuffix, []byte("compress"), t)

	<-time.After(time.Millisecond * 10)
	fileCount(dir, 5, t)

	existsWithContent(filename, b4, t)
	existsWithContent(fourthFilename, b3, t)

	notExist(thirdFilename, t)

	exists(notLogFile, t)
	exists(notLogFileDir, t)
}

func TestCleanupExistingBackups(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestCleanupExistingBackups", t)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	// make 3 backup files
	data := []byte("data")
	backup := backupFile(dir)
	err := ioutil.WriteFile(backup, data, 0644)
	isNil(err, t)

	newFakeTime()

	backup = backupFile(dir)
	err = ioutil.WriteFile(backup+compressSuffix, data, 0644)
	isNil(err, t)

	newFakeTime()

	backup = backupFile(dir)
	err = ioutil.WriteFile(backup, data, 0644)
	isNil(err, t)

	// now create a primary log file with some data
	filename := logFile(dir)
	err = ioutil.WriteFile(filename, data, 0644)
	isNil(err, t)

	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()), WithMaxSize(10), WithMaxRemain(1))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()

	newFakeTime()
	b2 := []byte("11111111")
	n, err := l.Write(b2)
	isNil(err, t)
	equals(len(b2), n, t)

	<-time.After(time.Millisecond * 10)
	fileCount(dir, 2, t)
}

func TestMaxAge(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestCleanupExistingBackups", t)
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}()

	filename := logFile(dir)
	l, err := NewWriter(WithLogPath(dir), WithFilename(logName()), WithMaxSize(10), WithMaxAge(1))
	isNil(err, t)
	defer func() {
		err := l.Close()
		if err != nil {
			return
		}
	}()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)
	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()
	b2 := []byte("11111111")
	n, err = l.Write(b2)
	isNil(err, t)
	equals(len(b2), n, t)
	existsWithContent(backupFile(dir), b, t)

	<-time.After(10 * time.Millisecond)

	fileCount(dir, 2, t)
	existsWithContent(filename, b2, t)
	existsWithContent(backupFile(dir), b, t)

	newFakeTime()

	b3 := []byte("2222222")
	n, err = l.Write(b3)
	isNil(err, t)
	equals(len(b3), n, t)
	existsWithContent(filename, b3, t)
	existsWithContent(backupFile(dir), b2, t)

	<-time.After(10 * time.Millisecond)
	fileCount(dir, 2, t)
	existsWithContent(filename, b3, t)
	existsWithContent(backupFile(dir), b2, t)
}

func TestWrite(t *testing.T) {
	writer, _ := NewWriter(
		WithLogPath("E:\\public\\public_project\\files\\log"),
		WithFilename("all.log"),
		WithMaxRemain(20), // 保留 10 个文件
		WithMaxSize(10),   // 每个文件最大为 10M
		WithMaxAge(30),    // 保留天数
		WithCompress(),
		WithLocalTime(),
		WithTimeRolling(),
	)
	_, _ = fmt.Fprintf(writer, "now :%s \n", time.Now().Format("2006-01-02T15-04-05.000"))
	//wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		go func(int) {
			for {
				_, err := fmt.Fprintf(writer, "now :%s \n", time.Now().Format("2006-01-02T15-04-05.000"))
				if err != nil {
					return
				}
			}
		}(i)
	}
	select {}
	//wg.Wait()
}

// makeTempDir creates a file with a semi-unique name in the OS temp directory.
// It should be based on the name of the test, to keep parallel tests from
// colliding, and must be cleaned up after the test is finished.
func makeTempDir(name string, t testing.TB) string {
	dir := time.Now().Format(name + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	isNilUp(os.Mkdir(dir, 0700), t, 1)
	return dir
}

// existsWithContent checks that the given file exists and has the correct content.
func existsWithContent(path string, content []byte, t testing.TB) {
	info, err := os.Stat(path)
	isNilUp(err, t, 1)
	equalsUp(int64(len(content)), info.Size(), t, 1)

	b, err := ioutil.ReadFile(path)
	isNilUp(err, t, 1)
	equalsUp(content, b, t, 1)
}

// logFile returns the log file name in the given directory for the current fake
// time.
func logFile(dir string) string {
	return filepath.Join(dir, logName())
}

// logName returns the log file name in the given directory for the current fake
// time.
func logName() string {
	return "foobar.log"
}

func backupFile(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().UTC().Format(backupTimeFormat)+".log")
}

func backupFileLocal(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().Format(backupTimeFormat)+".log")
}

// fileCount checks that the number of files in the directory is exp.
func fileCount(dir string, exp int, t testing.TB) {
	files, err := ioutil.ReadDir(dir)
	isNilUp(err, t, 1)
	// Make sure no other files were created
	equalsUp(exp, len(files), t, 1)
}

// newFakeTime sets the fake "current time" to two days later
func newFakeTime() {
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24 * 2)
}

func notExist(path string, t testing.TB) {
	_, err := os.Stat(path)
	assertUp(os.IsNotExist(err), t, 1, "expected to get os.IsNotExist, but instead got %v", err)
}

func exists(path string, t testing.TB) {
	_, err := os.Stat(path)
	assertUp(err == nil, t, 1, "expected file to exist, but got error from os.Stat: %v", err)
}
