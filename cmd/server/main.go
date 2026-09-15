package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/fumin/line"
	"github.com/fumin/line/config"
	"github.com/fumin/line/util"
)

var (
	cfgName = flag.String("c", "dev", "configuration name")
)

func runDailyTPE(hour, min int, fn func() error) {
	runDaily(hour, min, util.TaipeiTZ, fn)
}

func runDaily(hour, min int, location *time.Location, fn func() error) {
	if err := fn(); err != nil {
		log.Fatalf("%+v", err)
	}

	go func() {
		for {
			now := time.Now().In(location)
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, location)
			if nextRun.Before(now) {
				nextRun = nextRun.AddDate(0, 0, 1)
			}
			<-time.After(nextRun.Sub(now))

			if err := fn(); err != nil {
				log.Printf("%+v", err)
			}
		}
	}()
}

func mainWithErr() error {
	// Read config.
	var cfg config.Config
	var err error
	switch *cfgName {
	case "prod":
		cfg, err = config.Prod()
	default:
		cfg, err = config.Dev()
	}
	if err != nil {
		return errors.Wrap(err, "")
	}

	// Create server
	s, err := line.NewServer(cfg)
	if err != nil {
		return errors.Wrap(err, "")
	}
	defer s.Close()

	runDailyTPE(0, 1, func() error { return line.DeleteOldLogs(s.Root) })

	log.Printf("Listening on %s", s.Server.Addr)
	if err := s.Server.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("%+v", err)
	}

	return nil
}

func main() {
	flag.Parse()
	log.SetFlags(log.Lmicroseconds | log.Llongfile | log.LstdFlags)

	if err := mainWithErr(); err != nil {
		log.Fatalf("%+v", err)
	}
}
