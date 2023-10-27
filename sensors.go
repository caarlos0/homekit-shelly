package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	"github.com/charmbracelet/log"
)

type FloodSensor struct {
	*accessory.A
	Leak        *service.LeakSensor
	Temperature *service.TemperatureSensor
	Battery     *service.BatteryService

	topicBattery, topicFlood, topicTemperature, topicError, topicActReasons string
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
	a.topicBattery = fmt.Sprintf("shellies/shellyflood-%s/sensor/battery", id)
	a.topicFlood = fmt.Sprintf("shellies/shellyflood-%s/sensor/flood", id)
	a.topicTemperature = fmt.Sprintf("shellies/shellyflood-%s/sensor/temperature", id)
	a.topicError = fmt.Sprintf("shellies/shellyflood-%s/sensor/error", id)
	a.topicActReasons = fmt.Sprintf("shellies/shellyflood-%s/sensor/act_reasons", id)

	return &a
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (a *FloodSensor) Update(topic string, payload []byte) error {
	serial := a.Info.SerialNumber.Value()
	switch topic {
	case a.topicBattery:
		level, err := parseInt(payload)
		if err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		if err := a.Battery.BatteryLevel.SetValue(level); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		if err := a.Battery.StatusLowBattery.SetValue(boolToInt(level < 10)); err != nil {
			return fmt.Errorf("set battery status for %s: %w", serial, err)
		}
		log.Info("updated battery status", "shelly", serial, "status", level)
	case a.topicFlood:
		status := parseBool(payload)
		if err := a.Leak.LeakDetected.SetValue(status); err != nil {
			return fmt.Errorf("set leak status for %s: %w", a.Info.SerialNumber.Value(), err)
		}
		log.Info("updated leak status", "shelly", serial, "status", status)
	case a.topicTemperature:
		temp, err := parseFloat(payload)
		if err != nil {
			return fmt.Errorf("set temperature for %s: %w", a.Info.SerialNumber.Value(), err)
		}
		a.Temperature.CurrentTemperature.SetValue(temp)
		log.Info("updated temperature", "shelly", serial, "status", temp)
	case a.topicError:
		if string(payload) != "0" {
			log.Error(
				"sensor reported an error",
				"id", a.Info.SerialNumber,
				"err", string(payload),
			)
		}
	case a.topicActReasons:
		reasons, err := parseStringArray(payload)
		if err != nil {
			log.Error(
				"failed to parse sensor act reasons",
				"id",
				a.Info.SerialNumber,
				"payload", string(payload),
				"err", err,
			)
		}
		log.Info("sensor reason to act", "id", a.Info.SerialNumber.Value(), "reasons", reasons)
	}
	return nil
}

func parseFloat(b []byte) (float64, error) {
	return strconv.ParseFloat(string(b), 64)
}

func parseBool(b []byte) int {
	if string(b) == "true" {
		return characteristic.LeakDetectedLeakDetected
	}
	return characteristic.LeakDetectedLeakNotDetected
}

func parseInt(b []byte) (int, error) {
	return strconv.Atoi(string(b))
}

func parseStringArray(b []byte) ([]string, error) {
	var reasons []string
	err := json.Unmarshal(b, &reasons)
	return reasons, err
}
