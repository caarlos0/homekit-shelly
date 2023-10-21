package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v9"
	"github.com/charmbracelet/log"
)

type Config struct {
	Addr string `env:"LISTEN" envDefault:":8089"`
}

func main() {
	bridge := accessory.NewBridge(accessory.Info{
		Name:         "Shelly Bridge",
		Manufacturer: "Shelly",
	})

	leak1 := NewLeakSensor(accessory.Info{
		Name:         "Leak 1",
		Manufacturer: "Shelly",
	})

	leak2 := NewLeakSensor(accessory.Info{
		Name:         "Leak 2",
		Manufacturer: "Shelly",
	})

	smoke1 := NewSmokeSensor(accessory.Info{
		Name:         "Smoke 1",
		Manufacturer: "Shelly",
	})

	fs := hap.NewFsStore("./db")

	server, err := hap.NewServer(fs, bridge.A, leak1.A, leak2.A, smoke1.A)
	if err != nil {
		log.Fatal(err)
	}

	for i, a := range []*LeakSensor{leak1, leak2} {
		server.ServeMux().
			HandleFunc(fmt.Sprintf("/leak/%d/detected", i+1), func(_ http.ResponseWriter, _ *http.Request) {
				log.Info("status", "sensor", a.Name(), "leaking", true)
				a.LeakSensor.LeakDetected.SetValue(characteristic.LeakDetectedLeakDetected)
			})
		server.ServeMux().
			HandleFunc(fmt.Sprintf("/leak/%d/cleared", i+1), func(_ http.ResponseWriter, _ *http.Request) {
				log.Info("status", "sensor", a.Name(), "leaking", false)
				a.LeakSensor.LeakDetected.SetValue(characteristic.LeakDetectedLeakNotDetected)
			})
	}

	server.ServeMux().
		HandleFunc(fmt.Sprintf("/smoke/%d/detected", 1), func(_ http.ResponseWriter, _ *http.Request) {
			log.Info("status", "sensor", smoke1.Name(), "smoke", true)
			smoke1.SmokeSensor.SmokeDetected.SetValue(characteristic.SmokeDetectedSmokeDetected)
		})
	server.ServeMux().
		HandleFunc(fmt.Sprintf("/smoke/%d/cleared", 1), func(_ http.ResponseWriter, _ *http.Request) {
			log.Info("status", "sensor", smoke1.Name(), "smoke", false)
			smoke1.SmokeSensor.SmokeDetected.SetValue(characteristic.SmokeDetectedSmokeNotDetected)
		})

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("cannot parse config", "err", err)
	}

	server.Addr = cfg.Addr

	log.Info("server started", "addr", server.Addr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		signal.Stop(c)
		cancel()
	}()

	// Run the server.
	server.ListenAndServe(ctx)
}
