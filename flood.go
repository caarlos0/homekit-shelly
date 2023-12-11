package main

import (
	"encoding/json"
	"fmt"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type FloodEvent struct {
	Flood bool `json:"flood"`
	Tmp   Tmp  `json:"tmp"`
	Bat   Bat  `json:"bat"`
}

type Tmp struct {
	Value float64 `json:"value"`
}

type Bat struct {
	Value int `json:"value"`
}

type FloodSensor struct {
	*accessory.A
	Leak        *service.LeakSensor
	Temperature *service.TemperatureSensor
	Battery     *service.BatteryService

	topic string
}

func NewFloodSensor(info accessory.Info) *FloodSensor {
	a := FloodSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Leak = service.NewLeakSensor()
	a.AddS(a.Leak.S)

	a.Temperature = service.NewTemperatureSensor()
	a.AddS(a.Temperature.S)

	a.Battery = service.NewBatteryService()
	a.AddS(a.Battery.S)

	id := info.SerialNumber
	a.topic = fmt.Sprintf("shellies/shellyflood-%s/info", id)

	return &a
}

func (a *FloodSensor) listen(cli mqtt.Client, fs hap.Store) {
	serial := a.Info.SerialNumber.Value()
	cli.Subscribe(a.topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		msg.Ack()
		if len(msg.Payload()) == 0 {
			return
		}
		if err := fs.Set(cacheKey(a.topic), msg.Payload()); err != nil {
			log.Warn(
				"could not store event in cache",
				"type", "shellyflood",
				"shelly", serial,
				"err", err,
				"payload", string(msg.Payload()),
			)
		}
		var event FloodEvent
		if err := json.Unmarshal(msg.Payload(), &event); err != nil {
			log.Error(
				"could not parse shelly event",
				"type", "shellyflood",
				"shelly", serial,
				"err", err,
				"payload", string(msg.Payload()),
			)
			return
		}
		if err := a.Update(event); err != nil {
			log.Error(
				"could not update shelly",
				"type", "shellyflood",
				"shelly", serial,
				"err", err,
			)
		}
	})
}

func (a *FloodSensor) Update(evt FloodEvent) error {
	serial := a.Info.SerialNumber.Value()

	if v := evt.Bat.Value; a.Battery.BatteryLevel.Value() != v {
		if err := a.Battery.BatteryLevel.SetValue(v); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		if err := a.Battery.StatusLowBattery.SetValue(boolToInt(v < 10)); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		log.Info("updated battery status", "type", "shellyflood", "shelly", serial, "status", v)
	}

	if v := evt.Flood; boolToInt(v) != a.Leak.LeakDetected.Value() {
		if err := a.Leak.LeakDetected.SetValue(boolToInt(v)); err != nil {
			return fmt.Errorf("set flood status for %s: %w", a.Info.SerialNumber.Value(), err)
		}
		log.Info("updated flood status", "type", "shellyflood", "shelly", serial, "status", v)
	}

	a.Temperature.CurrentTemperature.SetValue(evt.Tmp.Value)
	log.Info("updated temperature", "type", "shellyflood", "shelly", serial, "status", evt.Tmp.Value)

	return nil
}

/*
Client null received PUBLISH (d0, q0, r0, m0, 'shellies/shellyflood-244CAB42D00A/info', ... (671 bytes))
{"wifi_sta":{"connected":true,"ssid":"Becker IoT","ip":"192.168.107.71","rssi":-72},"cloud":{"enabled":true,"connected":false},"mqtt":{"connected":true},"time":"","unixtime":0,"serial":1,"has_update":false,"mac":"244CAB42D00A","cfg_changed_cnt":0,"actions_stats":{"skipped":0},"is_valid":true,"flood":false,"tmp":{"value":26.25,"units":"C","tC":26.25,"tF":79.25,"is_valid":true},"bat":{"value":100,"voltage":3.03},"act_reasons":["sensor"],"rain_sensor":false,"sensor_error":0,"update":{"status":"unknown","has_update":false,"new_version":"","old_version":"20230913-112632/v1.14.0-gcb84623"},"ram_total":52392,"ram_free":41196,"fs_size":233681,"fs_free":145580,"uptime":1}
*/
