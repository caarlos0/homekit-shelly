package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
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

type LeakSensor struct {
	*accessory.A
	LeakSensor service.LeakSensor
}

func NewLeakSensor(info accessory.Info) *LeakSensor {
	a := LeakSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.LeakSensor = *service.NewLeakSensor()
	a.AddS(a.LeakSensor.S)

	return &a
}
