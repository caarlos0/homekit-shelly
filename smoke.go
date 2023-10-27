package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type SmokeEvent struct {
	Src    string `json:"src"`
	Method string `json:"method"`
	Params Params `json:"params"`
}

type Battery struct {
	Percent int `json:"percent"`
}

type Devicepower0 struct {
	Battery Battery `json:"battery"`
}

type Smoke0 struct {
	ID    int  `json:"id"`
	Alarm bool `json:"alarm"`
	Mute  bool `json:"mute"`
}

type Wifi struct {
	StaIP  string `json:"sta_ip"`
	Status string `json:"status"`
	Ssid   string `json:"ssid"`
	Rssi   int    `json:"rssi"`
}
type Params struct {
	Devicepower0 Devicepower0 `json:"devicepower:0"`
	Smoke0       Smoke0       `json:"smoke:0"`
}

type SmokeSensor struct {
	*accessory.A
	Smoke   *service.SmokeSensor
	Battery *service.BatteryService

	topic string
}

/*
{"src":"shellyplussmoke-80646fd09ed4","dst":"shellyplussmoke-80646fd09ed4/events","method":"NotifyFullStatus","params":{"ts":1.33,"ble":{},"cloud":{"connected":false},"devicepower:0":{"id": 0,"battery":{"V":2.99, "percent":97}},"mqtt":{"connected":true},"smoke:0":{"id":0,"alarm":false, "mute":false},"sys":{"mac":"80646FD09ED4","restart_required":false,"time":null,"unixtime":null,"uptime":1,"ram_size":245908,"ram_free":163788,"fs_size":458752,"fs_free":188416,"cfg_rev":14,"kvs_rev":0,"webhook_rev":0,"available_updates":{},"wakeup_reason":{"boot":"poweron","cause":"button"},"wakeup_period":86400},"wifi":{"sta_ip":"192.168.107.23","status":"got ip","ssid":"Becker IoT","rssi":-20},"ws":{"connected":false}}}

{"src":"shellyplussmoke-80646fd09ed4","dst":"shellyplussmoke-80646fd09ed4/events","method":"NotifyFullStatus","params":{"ts":1.32,"ble":{},"cloud":{"connected":false},"devicepower:0":{"id": 0,"battery":{"V":2.99, "percent":97}},"mqtt":{"connected":true},"smoke:0":{"id":0,"alarm":false, "mute":false},"sys":{"mac":"80646FD09ED4","restart_required":false,"time":null,"unixtime":null,"uptime":1,"ram_size":245944,"ram_free":166000,"fs_size":458752,"fs_free":188416,"cfg_rev":14,"kvs_rev":0,"webhook_rev":0,"available_updates":{},"wakeup_reason":{"boot":"poweron","cause":"button"},"wakeup_period":86400},"wifi":{"sta_ip":"192.168.107.23","status":"got ip","ssid":"Becker IoT","rssi":-38},"ws":{"connected":false}}}

*/

func NewSmokeSensor(info accessory.Info) *SmokeSensor {
	a := SmokeSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Smoke = service.NewSmokeSensor()
	a.AddS(a.Smoke.S)

	a.Battery = service.NewBatteryService()
	a.AddS(a.Battery.S)

	id := info.SerialNumber
	a.topic = fmt.Sprintf("shellyplussmoke-%s/events", strings.ToLower(id))
	return &a
}

func (a *SmokeSensor) listen(cli mqtt.Client, fs hap.Store) {
	log.Info("topic is: " + a.topic)
	cli.Subscribe(a.topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		msg.Ack()
		if len(msg.Payload()) == 0 {
			return
		}
		if err := fs.Set(cacheKey(a.topic), msg.Payload()); err != nil {
			log.Warn("could not store event in cache", "err", err, "payload", string(msg.Payload()))
		}
		var event SmokeEvent
		if err := json.Unmarshal(msg.Payload(), &event); err != nil {
			log.Error("could not parse shelly event", "err", err, "payload", string(msg.Payload()))
			return
		}
		if err := a.Update(event); err != nil {
			log.Error("could not update shelly", "err", err)
		}
	})
}

func (a *SmokeSensor) Update(evt SmokeEvent) error {
	serial := a.Info.SerialNumber.Value()

	if v := evt.Params.Devicepower0.Battery.Percent; a.Battery.BatteryLevel.Value() != v {
		if err := a.Battery.BatteryLevel.SetValue(v); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		if err := a.Battery.StatusLowBattery.SetValue(boolToInt(v < 10)); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		log.Info("updated battery status", "shelly", serial, "status", v)
	}

	if v := evt.Params.Smoke0.Alarm; boolToInt(v) != a.Smoke.SmokeDetected.Value() {
		if err := a.Smoke.SmokeDetected.SetValue(boolToInt(v)); err != nil {
			return fmt.Errorf("set smoke status for %s: %w", a.Info.SerialNumber.Value(), err)
		}
		log.Info("updated smoke status", "shelly", serial, "status", v)
	}

	return nil
}
