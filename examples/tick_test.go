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
	tick := periodic.NewTask("tick", time.Second, periodic.Seq(tick, tack))
	tick.SetLog(stdout)
	tick.Start()
	time.Sleep(5 * time.Second)
	tick.Stop()
}
