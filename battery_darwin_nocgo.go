// battery
// Copyright (C) 2016 Karol 'Kenji Takahashi' Woźniak
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
// OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
// TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE
// OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

// +build darwin

package battery

import (
	"errors"
	"math"
	"os/exec"
	"strconv"

	"github.com/DHowett/go-plist"
)

type battery struct {
	Location          int
	CurrentCapacity   int
	MaxCapacity       int
	DesignCapacity    int
	Amperage          int64
	FullyCharged      bool
	IsCharging        bool
	ExternalConnected bool
}

func readBatteries() ([]*battery, error) {
	out, err := exec.Command("ioreg", "-n", "AppleSmartBattery", "-r", "-a").Output()
	if err != nil {
		return nil, err
	}

	var data []*battery
	if _, err = plist.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func convertBattery(battery *battery) *Battery {
	b := &Battery{
		Name:       strconv.Itoa(battery.Location),
		Current:    float64(battery.CurrentCapacity),
		Full:       float64(battery.MaxCapacity),
		Design:     float64(battery.DesignCapacity),
		ChargeRate: math.Abs(float64(battery.Amperage)),
	}
	switch {
	case !battery.ExternalConnected:
		b.State, _ = newState("Discharging")
	case battery.IsCharging:
		b.State, _ = newState("Charging")
	case battery.CurrentCapacity == 0:
		b.State, _ = newState("Empty")
	case battery.FullyCharged:
		b.State, _ = newState("Full")
	default:
		b.State, _ = newState("Unknown")
	}
	return b
}

func get(idx int) (*Battery, error) {
	batteries, err := readBatteries()
	if err != nil {
		return nil, FatalError{Err: err}
	}

	for _, battery := range batteries {
		if battery.Location == idx {
			return convertBattery(battery), nil
		}
	}
	return nil, FatalError{Err: errors.New("Not found")}
}

func getAll() ([]*Battery, error) {
	_batteries, err := readBatteries()
	if err != nil {
		return nil, FatalError{Err: err}
	}

	batteries := make([]*Battery, len(_batteries))
	for i, battery := range _batteries {
		batteries[i] = convertBattery(battery)
	}
	return batteries, nil
}
