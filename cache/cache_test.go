package cache

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
)

var defaultData []byte = []byte("\x00\x01foo\x00bar\x00")

//Test case to check if read after write is same or not
func TestReadAfterWrite(t *testing.T) {
	Write("testCacheFile", []byte("\x00\x01foo\x00bar\x00"))
	byteArr := Read("testCacheFile")

	if !reflect.DeepEqual([]byte("\x00\x01foo\x00bar\x00"), byteArr) {
		t.Errorf("Read after write failed")
	}
}

func TestDuplicateWrite(t *testing.T) {
	//stdoutWriter := io.Writer(os.Stdout)
	//logs.InitializeWithRequestAndApplicationLogger(&stdoutWriter, &stdoutWriter)
	Write("testCacheFile1", []byte("\x00\x01foo\x00bar\x00"))
	err := Write("testCacheFile1", []byte("\x00\x01foo\x00bar\x00"))

	if err == nil {
		t.Errorf("Duplicate write request failed")
	}
}

func concurrentRead(filePrefix string, wg *sync.WaitGroup) {
	for nThreads := 0; nThreads < 3; nThreads++ {
		go func(filePrefix string, wg *sync.WaitGroup) {
			defer (*wg).Done()
			for nReads := 0; nReads < 1000; nReads++ {
				Read(filePrefix + strconv.Itoa(nReads%10))
			}
		}(filePrefix, wg)
	}
}

func concurrentWrite(filePrefix string, data []byte, wg *sync.WaitGroup) {
	for nThreads := 0; nThreads < 3; nThreads++ {
		go func(filePrefix string, data []byte, wg *sync.WaitGroup) {
			defer (*wg).Done()
			for nWrites := 0; nWrites < 1000; nWrites++ {
				Write(filePrefix+strconv.Itoa(nWrites%10), data)
			}
		}(filePrefix, data, wg)
	}
}

func TestConcurrentReadAndWrite(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(6)
	concurrentRead("concurrentTest", &wg)
	concurrentWrite("concurrentTest", defaultData, &wg)
	wg.Wait()
}
