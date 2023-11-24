package misc

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase64Concurrency(t *testing.T) {
	str := "hello world"
	out := B64EncStr([]byte(str))
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			dst := B64Enc([]byte(str))
			assert.Equal(t, []byte(out), dst)
			wg.Done()
		}()
	}
	wg.Wait()
	t.Log("Over")
}
