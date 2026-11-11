package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v10"
	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Config struct {
	BrokerHost string   `env:"MQTT_HOST" envDefault:"localhost"`
	BrokerPort int      `env:"MQTT_PORT" envDefault:"1883"`
	Floods     []string `env:"FLOODS"`
	Smokes     []string `env:"SMOKES"`
	HTs        []string `env:"HTS"`

	BrokerTLS      *bool  `env:"MQTT_TLS"`
	BrokerUsername string `env:"MQTT_USERNAME"`
	BrokerPassword string `env:"MQTT_PASSWORD"`
}

func main() {
	log.Info(
		"homekit-shelly",
		"version", version,
		"commit", commit,
		"date", date,
		"info", strings.Join([]string{
			"Homekit bridge for Shelly devices",
			"© Carlos Alexandro Becker",
			"https://becker.software",
		}, "\n"),
	)

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("cannot parse config", "err", err)
	}

	log.Info("loading accessories", "smokes", cfg.Smokes, "floods", cfg.Floods, "h&ts", cfg.HTs)

	fs := hap.NewFsStore("./db")

	bridge := accessory.NewBridge(accessory.Info{
		Name:         "Shelly Bridge",
		Manufacturer: "Shelly",
	})

	cli := mqtt.NewClient(mqttClientOptions(cfg))
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

	hts := make([]*HTSensor, len(cfg.HTs))
	for i, id := range cfg.HTs {
		a := NewHTSensor(accessory.Info{
			Name:         fmt.Sprintf("H&T %d", i+1),
			Manufacturer: "Shelly",
			Model:        "Plus H&T",
			SerialNumber: id,
		})
		_ = a.Battery.ChargingState.SetValue(characteristic.ChargingStateNotChargeable)

		a.listen(cli, fs)

		// try to publish cached status
		cache, _ := fs.Get(cacheKey(a.topic))
		_ = cli.Publish(a.topic, 1, false, cache)

		hts[i] = a
	}

	server, err := hap.NewServer(fs, bridge.A, allAccessories(floods, smokes, hts)...)
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

func mqttClientOptions(cfg Config) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	ip := net.ParseIP(cfg.BrokerHost)
	useTLS := !strings.EqualFold(cfg.BrokerHost, "localhost") && (ip == nil || !ip.IsLoopback())
	if cfg.BrokerTLS != nil {
		useTLS = *cfg.BrokerTLS
	}
	scheme := "tcp"
	if useTLS {
		scheme = "ssl"
		opts.SetTLSConfig(&tls.Config{ServerName: cfg.BrokerHost})
	}
	opts.AddBroker(scheme + "://" + net.JoinHostPort(cfg.BrokerHost, strconv.Itoa(cfg.BrokerPort)))
	opts.SetClientID("homekit_shelly")
	opts.SetUsername(cfg.BrokerUsername)
	opts.SetPassword(cfg.BrokerPassword)
	opts.OnConnect = func(_ mqtt.Client) {
		log.Info("connected to mqtt", "host", cfg.BrokerHost, "port", cfg.BrokerPort)
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Error("connection to mqtt lost", "err", err)
	}
	return opts
}

func allAccessories(floods []*FloodSensor, smokes []*SmokeSensor, hts []*HTSensor) []*accessory.A {
	var r []*accessory.A
	for _, a := range floods {
		r = append(r, a.A)
	}
	for _, a := range smokes {
		r = append(r, a.A)
	}
	for _, a := range hts {
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
