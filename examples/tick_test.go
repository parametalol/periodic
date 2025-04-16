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

var stdout *stdoutLog

func tick(_ context.Context) error {
	stdout.Info("tick")
	return nil
}

func tack(_ context.Context) error {
	stdout.Info("tack")
	return nil
}

func TestTick(t *testing.T) {
	tick := periodic.NewTask("tick-tack", time.Second,
		periodic.WithLog(stdout, periodic.Seq(tick, tack)))
	tick.Start()
	time.Sleep(2500 * time.Millisecond)
	// The tick-tack will be called 3 times:
	// 1: on Start()
	// 2: after second 1
	// 3: after second 2
	tick.Stop()
	tick.Wait()
}
