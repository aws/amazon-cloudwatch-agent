package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/open-telemetry/opamp-go/internal/examples/supervisor/supervisor"
)

func main() {
	logger := &supervisor.Logger{Logger: log.Default()}
	supervisor, err := supervisor.NewSupervisor(logger)
	if err != nil {
		logger.Errorf(context.Background(), err.Error())
		os.Exit(-1)
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	supervisor.Shutdown()
}
