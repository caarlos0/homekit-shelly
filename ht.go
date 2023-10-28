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

// TODO: no docs about this it seems, need one to test
type HTEvent struct {
	Method  string
	Battery int     // XXX: change
	Hr      float64 // XXX: change
	Temp    float64 // XXX: change
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
	a.topic = fmt.Sprintf("shellyplush&t-%s/events/rpc", strings.ToLower(id))
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

	if v := evt.Battery; a.Battery.BatteryLevel.Value() != v {
		if err := a.Battery.BatteryLevel.SetValue(v); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		if err := a.Battery.StatusLowBattery.SetValue(boolToInt(v < 10)); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		log.Info("updated battery status", "type", "shellyplush&t", "shelly", serial, "status", v)
	}

	if v := evt.Temp; a.Temperature.CurrentTemperature.Value() != v {
		a.Temperature.CurrentTemperature.SetValue(v)
		log.Info("updated temperature", "type", "shellyplush&t", "shelly", serial, "status", v)
	}

	if v := evt.Hr; a.Humidity.Value() != v {
		a.Humidity.SetValue(v)
		log.Info("updated temperature", "type", "shellyplush&t", "shelly", serial, "status", v)
	}

	return nil
}
