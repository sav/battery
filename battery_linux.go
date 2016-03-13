package battery

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
)

const sysfs = "/sys/class/power_supply"

type NotBatteryError struct{}

func (n NotBatteryError) Error() string {
	return "Not battery"
}

func readFloat(sysfs, filename string) (float64, error) {
	str, err := ioutil.ReadFile(filepath.Join(sysfs, filename))
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(string(str[:len(str)-1]), 64)
}

func getByName(name string) (*Battery, error) {
	sysfs := filepath.Join("/sys/class/power_supply", name)
	t, err := ioutil.ReadFile(filepath.Join(sysfs, "type"))
	if err != nil {
		return nil, FatalError{Err: err}
	}
	if string(t) != "Battery\n" {
		return nil, NotBatteryError{}
	}

	b := &Battery{Name: name}
	e := &PartialError{}
	b.Current, e.Current = readFloat(sysfs, "energy_now")
	b.Full, e.Full = readFloat(sysfs, "energy_full")
	b.Design, e.Design = readFloat(sysfs, "energy_full_design")
	b.ChargeRate, e.ChargeRate = readFloat(sysfs, "power_now")
	state, err := ioutil.ReadFile(filepath.Join(sysfs, "status"))
	if err == nil {
		b.State, e.State = newState(string(state[:len(state)-1]))
	} else {
		e.State = err
	}

	if !e.Nil() {
		return b, e
	}
	return b, nil
}

func get(idx int) (*Battery, error) {
	return getByName(fmt.Sprintf("BAT%d", idx))
}

func getAll() ([]*Battery, error) {
	files, err := ioutil.ReadDir("/sys/class/power_supply")
	if err != nil {
		return nil, FatalError{Err: err}
	}

	batteries := []*Battery{}
	errors := Errors{}
	for _, file := range files {
		battery, err := getByName(file.Name())
		switch err.(type) {
		case NotBatteryError:
			continue
		case FatalError:
			batteries = append(batteries, nil)
			errors = append(errors, err)
		default:
			batteries = append(batteries, battery)
			errors = append(errors, err)
		}
	}

	if errors.Nil() {
		return batteries, nil
	}
	return batteries, errors
}
