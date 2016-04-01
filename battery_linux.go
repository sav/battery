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

package battery

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
)

const sysfsRoot = "/sys/class/power_supply"

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
	sysfs := filepath.Join(sysfsRoot, name)
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
	files, err := ioutil.ReadDir(sysfsRoot)
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
