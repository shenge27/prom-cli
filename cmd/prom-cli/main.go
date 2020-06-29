package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shenge27/prom-cli/command"
)

func main() {
	app := &command.App{
		Context: contextProcess(),
	}

	rootCmd := NewCommand(app)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func contextProcess() context.Context {
	ch := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
	}()
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	return ctx
}
