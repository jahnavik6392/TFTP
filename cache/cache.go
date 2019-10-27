package cache

import (
	"errors"
	"sync"
)

var m = make(map[string][]byte)
var lock = sync.RWMutex{}

func Read(fileName string) (byteArr []byte) {
	lock.RLock()
	defer lock.RUnlock()
	return m[fileName]
}

func Write(fileName string, byteArr []byte) (err error) {
	lock.Lock()
	defer lock.Unlock()
	if m[fileName] != nil {
		err = errors.New("Duplicate write request")
		return err
	}
	m[fileName] = byteArr
	return nil
}
