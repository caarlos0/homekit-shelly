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

type SmokeSensor struct {
	*accessory.A
	SmokeSensor service.SmokeSensor
}

func NewSmokeSensor(info accessory.Info) *SmokeSensor {
	a := SmokeSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.SmokeSensor = *service.NewSmokeSensor()
	a.AddS(a.SmokeSensor.S)

	return &a
}

type FloodSensor struct {
	*accessory.A
	LeakSensor  *service.LeakSensor
	Temperature *service.TemperatureSensor
	Battery     *service.BatteryService

	topicBattery, topicFlood, topicTemperature, topicError, topicActReasons string
}

func NewFloodSensor(info accessory.Info) *FloodSensor {
	a := FloodSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.LeakSensor = service.NewLeakSensor()
	a.AddS(a.LeakSensor.S)

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

func (a *FloodSensor) Update(topic string, payload []byte) error {
	switch topic {
	case a.topicBattery:
		level, err := parseBattery(payload)
		if err != nil {
			return fmt.Errorf("set battery status for %s: %w", a.Info.SerialNumber.Value(), err)
		}
		if err := a.Battery.BatteryLevel.SetValue(level); err != nil {
			return fmt.Errorf("set battery status for %s: %w", a.Info.SerialNumber.Value(), err)
		}
	case a.topicFlood:
		if err := a.LeakSensor.LeakDetected.SetValue(parseFlood(payload)); err != nil {
			return fmt.Errorf("set leak status for %s: %w", a.Info.SerialNumber.Value(), err)
		}
	case a.topicTemperature:
		temp, err := parseTemp(payload)
		if err != nil {
			return fmt.Errorf("set temperature for %s: %w", a.Info.SerialNumber.Value(), err)
		}
		a.Temperature.CurrentTemperature.SetValue(temp)
	case a.topicError:
		if string(payload) != "0" {
			log.Error(
				"sensor reported an error",
				"id",
				a.Info.SerialNumber,
				"err",
				string(payload),
			)
		}
	case a.topicActReasons:
		reasons, err := parseActReason(payload)
		if err != nil {
			log.Error(
				"failed to parse sensor act reasons",
				"id",
				a.Info.SerialNumber,
				"err",
				string(payload),
				"err",
				err,
			)
		}
		log.Info("sensor reason to act", "id", a.Info.SerialNumber.Value(), "reasons", reasons)
	}
	return nil
}

func parseTemp(b []byte) (float64, error) {
	return strconv.ParseFloat(string(b), 64)
}

func parseFlood(b []byte) int {
	if string(b) == "true" {
		return characteristic.LeakDetectedLeakDetected
	}
	return characteristic.LeakDetectedLeakNotDetected
}

func parseBattery(b []byte) (int, error) {
	return strconv.Atoi(string(b))
}

func parseActReason(b []byte) ([]string, error) {
	var reasons []string
	err := json.Unmarshal(b, &reasons)
	return reasons, err
}
