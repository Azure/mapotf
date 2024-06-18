package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/mapotf/cmd"
)

// TODO: investigate dynamic merge with dynamic
func main() {
	mptfArgs, nonMptfArgs := cmd.FilterArgs(os.Args)
	os.Args = mptfArgs
	cmd.NonMptfArgs = nonMptfArgs
	ctx, cancelFunc := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancelFunc()
	}()
	cmd.Execute(ctx)
}
