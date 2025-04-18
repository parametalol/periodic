package examples

import (
	"sync"
	"testing"
	"time"

	"parameta.lol/periodic"
)

var i int

func counter() {
	i++
}

func TestTestTicker(t *testing.T) {
	ticker := periodic.NewTestTicker(0)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go periodic.Routine(&wg, ticker.TickChan(), counter)

	for range 3 {
		ticker.(periodic.TestTicker) <- time.Now()
	}
	ticker.Destroy()
	wg.Wait()

	if i != 3 {
		t.Errorf("expected 3 ticks, got %d", i)
	}
}
