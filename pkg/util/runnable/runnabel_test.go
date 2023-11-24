package runnable

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"
)

type printer struct {
	name string
}

func (p *printer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("finish ", p.name)
			return nil
		default:
			log.Println(p.name)
			time.Sleep(time.Second)
		}
	}

	return fmt.Errorf("test error")
}

func TestRunnableGroup(t *testing.T) {
	var errCh = make(chan error)
	group := NewRunnableGroup(errCh)
	group.AddDefault(&printer{name: "jerry"})
	group.AddDefault(&printer{name: "tom"})
	err := group.Start(context.Background())
	t.Log(err)

	go func() {
		select {
		case err := <-errCh:
			log.Println(err)
			return
		}
	}()

	time.Sleep(3 * time.Second)
	group.StopAndWait(context.Background())
}
