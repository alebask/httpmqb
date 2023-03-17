package main

import (
	"flag"
	"httpmqb/httpmq"
	"httpmqb/logger"
	"net/http"
	"os"
	"strconv"
)

func main() {

	var port int
	flag.IntVar(&port, "port", 5000, "listening port")
	flag.Parse()

	var portFlagSet bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portFlagSet = true
		}
	})

	if !portFlagSet {
		sport := os.Getenv("HTTPMQB_PORT")
		if sport != "" {
			envport, err := strconv.Atoi(sport)
			if err != nil {
				logger.Error("environment variable HTTPMQB_PORT is invalid", logger.Fields{"port": port, "HTTPMQB_PORT": sport})
			} else {
				port = envport
			}
		}
	}

	logger.Info("httpmqb started", logger.Fields{"port": port})

	err := httpmq.New().ListenAndServe(port)

	if err != nil && err != http.ErrServerClosed {
		logger.Error("unexpected error", logger.Fields{"error": err})
	}

	logger.Info("httpmqb stopped", logger.Fields{"port": port})
}
