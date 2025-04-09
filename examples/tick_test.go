package examples

import (
	"context"
	"fmt"
	"testing"
	"time"

	"parameta.lol/periodic"
)

type stdoutLog struct{}

func (sl *stdoutLog) Info(args ...any) {
	fmt.Print("INFO: ")
	fmt.Println(args...)
}

func (sl *stdoutLog) Error(args ...any) {
	fmt.Print("ERRR: ")
	fmt.Println(args...)
}

func TestTick(t *testing.T) {
	log := &stdoutLog{}
	tick := periodic.NewTask("tick", time.Second, func(ctx context.Context) error {
		log.Info("tick")
		return nil
	})
	tick.SetLog(log)
	tick.Start()
	time.Sleep(5 * time.Second)
	tick.Stop()
}
