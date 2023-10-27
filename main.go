package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v9"
	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	BrokerHost string   `env:"MQTT_HOST" envDefault:"localhost"`
	BrokerPort int      `env:"MQTT_PORT" envDefault:"1883"`
	Floods     []string `env:"FLOODS"`
	Smokes     []string `env:"SMOKES"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("cannot parse config", "err", err)
	}

	fs := hap.NewFsStore("./db")

	bridge := accessory.NewBridge(accessory.Info{
		Name:         "Shelly Bridge",
		Manufacturer: "Shelly",
	})

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.BrokerHost, cfg.BrokerPort))
	opts.SetClientID("homekit_shelly")
	opts.OnConnect = func(_ mqtt.Client) {
		log.Info("connected to mqtt")
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Error("connection to mqtt lost", "err", err)
	}
	cli := mqtt.NewClient(opts)
	if token := cli.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("could not connect to mqtt", "token", token)
	}

	floods := make([]*FloodSensor, len(cfg.Floods))
	for i, id := range cfg.Floods {
		a := NewFloodSensor(accessory.Info{
			Name:         fmt.Sprintf("Leak %d", i+1),
			Manufacturer: "Shelly",
			Model:        "Flood",
			SerialNumber: id,
		})
		_ = a.Battery.ChargingState.SetValue(characteristic.ChargingStateNotChargeable)

		a.listen(cli, fs)

		// try to publish cached status
		cache, _ := fs.Get(cacheKey(a.topic))
		_ = cli.Publish(a.topic, 1, false, cache)

		floods[i] = a
	}

	smokes := make([]*SmokeSensor, len(cfg.Smokes))
	for i, id := range cfg.Smokes {
		a := NewSmokeSensor(accessory.Info{
			Name:         fmt.Sprintf("Smoke %d", i+1),
			Manufacturer: "Shelly",
			Model:        "Plus Smoke",
			SerialNumber: id,
		})
		_ = a.Battery.ChargingState.SetValue(characteristic.ChargingStateNotChargeable)

		a.listen(cli, fs)

		// try to publish cached status
		cache, _ := fs.Get(cacheKey(a.topic))
		_ = cli.Publish(a.topic, 1, false, cache)

		smokes[i] = a
	}

	server, err := hap.NewServer(fs, bridge.A, allAccessories(floods, smokes)...)
	if err != nil {
		log.Fatal("fail to start server", "error", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		log.Info("stopping server...")
		signal.Stop(c)
		cancel()
	}()

	log.Info("starting server...")
	if err := server.ListenAndServe(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("failed to close server", "err", err)
	}
}

func allAccessories(floods []*FloodSensor, smokes []*SmokeSensor) []*accessory.A {
	var r []*accessory.A
	for _, a := range floods {
		r = append(r, a.A)
	}
	for _, a := range smokes {
		r = append(r, a.A)
	}
	return r
}

func cacheKey(topic string) string {
	return strings.ReplaceAll(topic, "/", "-")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
