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
		return nil, FatalError{Err: err}
	}

	var data []*battery
	if _, err = plist.Unmarshal(out, &data); err != nil {
		return nil, FatalError{Err: err}
	}
	return data, nil
}

func convertBattery(battery *battery) (*Battery, error) {
	var stateString string
	switch {
	case !battery.ExternalConnected:
		stateString = "Discharging"
	case battery.IsCharging:
		stateString = "Charging"
	case battery.CurrentCapacity == 0:
		stateString = "Empty"
	case battery.FullyCharged:
		stateString = "Full"
	default:
		stateString = "Unknown"
	}
	state, err := newState(stateString)

	b := &Battery{
		Name:       strconv.Itoa(battery.Location),
		State:      state,
		Current:    float64(battery.CurrentCapacity),
		Full:       float64(battery.MaxCapacity),
		Design:     float64(battery.DesignCapacity),
		ChargeRate: math.Abs(float64(battery.Amperage)),
	}

	if err != nil {
		return b, PartialError{State: err}
	}
	return b, nil
}

func get(idx int) (*Battery, error) {
	batteries, err := readBatteries()
	if err != nil {
		return nil, err
	}

	for _, battery := range batteries {
		if battery.Location == idx {
			return convertBattery(battery)
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
	errors := make(Errors, len(_batteries))
	for i, battery := range _batteries {
		batteries[i], errors[i] = convertBattery(battery)
	}
	if errors.Nil() {
		return batteries, nil
	}
	return batteries, errors
}
