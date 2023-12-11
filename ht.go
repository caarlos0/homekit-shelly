package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	"github.com/charmbracelet/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type HTEvent struct {
	Src    string   `json:"src"`
	Dst    string   `json:"dst"`
	Method string   `json:"method"`
	Params HTParams `json:"params"`
}

type HTBattery struct {
	Percent int `json:"percent"`
}

type HTDevicepower0 struct {
	ID      int       `json:"id"`
	Battery HTBattery `json:"battery"`
}

type HTHumidity0 struct {
	Rh float64 `json:"rh"`
}

type HTTemperature0 struct {
	TC float64 `json:"tC"`
}

type HTParams struct {
	Devicepower0 HTDevicepower0 `json:"devicepower:0"`
	Humidity0    HTHumidity0    `json:"humidity:0"`
	Temperature0 HTTemperature0 `json:"temperature:0"`
}

type HTSensor struct {
	*accessory.A
	Temperature *service.TemperatureSensor
	Humidity    *characteristic.CurrentRelativeHumidity
	Battery     *service.BatteryService

	topic string
}

func NewHTSensor(info accessory.Info) *HTSensor {
	a := HTSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Temperature = service.NewTemperatureSensor()
	a.AddS(a.Temperature.S)

	a.Humidity = characteristic.NewCurrentRelativeHumidity()
	a.Temperature.AddC(a.Humidity.C)

	a.Battery = service.NewBatteryService()
	a.AddS(a.Battery.S)

	id := info.SerialNumber

	a.topic = fmt.Sprintf("shellyplusht-%s/events/rpc", strings.ToLower(id))
	return &a
}

func (a *HTSensor) listen(cli mqtt.Client, fs hap.Store) {
	serial := a.Info.SerialNumber.Value()
	cli.Subscribe(a.topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		msg.Ack()
		if len(msg.Payload()) == 0 {
			return
		}
		if err := fs.Set(cacheKey(a.topic), msg.Payload()); err != nil {
			log.Warn(
				"could not store event in cache",
				"type", "shellyplussmoke",
				"shelly", serial,
				"err", err,
				"payload", string(msg.Payload()),
			)
		}
		var event HTEvent
		if err := json.Unmarshal(msg.Payload(), &event); err != nil {
			log.Error(
				"could not parse shelly event",
				"type", "shellyplussmoke",
				"shelly", serial,
				"err", err,
				"payload", string(msg.Payload()),
			)
			return
		}
		if err := a.Update(event); err != nil {
			log.Error("could not update shelly", "err", err)
		}
	})
}

func (a *HTSensor) Update(evt HTEvent) error {
	serial := a.Info.SerialNumber.Value()
	if evt.Method != "NotifyFullStatus" {
		log.Warn("ignoring event", "method", evt.Method)
		return nil
	}

	if v := evt.Params.Devicepower0.Battery.Percent; a.Battery.BatteryLevel.Value() != v {
		if err := a.Battery.BatteryLevel.SetValue(v); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		if err := a.Battery.StatusLowBattery.SetValue(boolToInt(v < 10)); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		log.Info("updated battery status", "type", "shellyplush&t", "shelly", serial, "status", v)
	}

	a.Temperature.CurrentTemperature.SetValue(evt.Params.Temperature0.TC)
	log.Info("updated temperature", "type", "shellyplush&t", "shelly", serial, "status", evt.Params.Temperature0.TC)

	a.Humidity.SetValue(evt.Params.Humidity0.Rh)
	log.Info("updated temperature", "type", "shellyplush&t", "shelly", serial, "status", evt.Params.Humidity0.Rh)

	return nil
}
