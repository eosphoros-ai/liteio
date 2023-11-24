package misc

import (
	"math/rand"
	"sync"
	"time"
)

const (
	CharSet = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// lower-case characters and numbers
	LowerCharNumSet = "abcdefghijklmnopqrstuvwxyz" + "0123456789"
)

var (
	// mutex to protect Random
	randomMutex sync.Mutex
	// random is not concurrently safe
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// RandomStringWithCharSet generates random string
func RandomStringWithCharSet(length int, charset string) string {
	b := make([]byte, length)
	randomMutex.Lock()
	for i := range b {
		b[i] = charset[random.Intn(len(charset))]
	}
	randomMutex.Unlock()
	return string(b)
}

// RandomIntn calls rand.Intn()
func RandomIntn(n int) (m int) {
	randomMutex.Lock()
	m = random.Intn(n)
	randomMutex.Unlock()
	return
}

// RandomInt calls rand.Int()
func RandomInt() (m int) {
	randomMutex.Lock()
	m = random.Int()
	randomMutex.Unlock()
	return
}
