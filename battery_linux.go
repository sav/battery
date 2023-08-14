// battery
// Copyright (C) 2016-2017,2023 Karol 'Kenji Takahashi' Woźniak
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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

const sysfs = "/sys/class/power_supply"

func readString(path, filename string) (string, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(path, filename))
	if err != nil {
		return "", err
	}
	return string(bytes[:len(bytes)-1]), nil
}

func readInt(path, filename string) (int64, error) {
	str, err := readString(path, filename)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func readFloat(path, filename string) (float64, error) {
	str, err := readString(path, filename)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func readMilli(path, filename string) (float64, error) {
	val, err := readFloat(path, filename)
	if err != nil {
		return 0, err
	}
	return val / 1000, nil // Convert micro->milli
}

func readAmp(path, filename string, volts float64) (float64, error) {
	val, err := readMilli(path, filename)
	if err != nil {
		return 0, err
	}
	return val * volts, nil
}

func isBattery(path string) bool {
	t, err := ioutil.ReadFile(filepath.Join(path, "type"))
	return err == nil && string(t) == "Battery\n"
}

func getBatteryFiles() ([]string, error) {
	files, err := ioutil.ReadDir(sysfs)
	if err != nil {
		return nil, err
	}

	var bFiles []string
	for _, file := range files {
		path := filepath.Join(sysfs, file.Name())
		if isBattery(path) {
			bFiles = append(bFiles, path)
		}
	}
	return bFiles, nil
}

func getByPath(path string) (*Battery, error) {
	b := &Battery{}
	e := ErrPartial{}
	b.Capacity, e.Capacity = readFloat(path, "capacity")
	b.Current, e.Current = readMilli(path, "energy_now")
	b.Voltage, e.Voltage = readMilli(path, "voltage_now")
	b.Voltage /= 1000

	b.DesignVoltage, e.DesignVoltage = readMilli(path, "voltage_max_design")
	if e.DesignVoltage != nil {
		b.DesignVoltage, e.DesignVoltage = readMilli(path, "voltage_min_design")
	}
	if e.DesignVoltage != nil && e.Voltage == nil {
		b.DesignVoltage, e.DesignVoltage = b.Voltage, nil
	}
	b.DesignVoltage /= 1000

	if os.IsNotExist(e.Current) {
		if e.DesignVoltage == nil {
			b.Design, e.Design = readAmp(path, "charge_full_design", b.DesignVoltage)
		} else {
			e.Design = e.DesignVoltage
		}
		if e.Voltage == nil {
			b.Current, e.Current = readAmp(path, "charge_now", b.Voltage)
			b.Full, e.Full = readAmp(path, "charge_full", b.Voltage)
			b.ChargeRate, e.ChargeRate = readAmp(path, "current_now", b.Voltage)
		} else {
			e.Current = e.Voltage
			e.Full = e.Voltage
			e.ChargeRate = e.Voltage
		}
	} else {
		b.Full, e.Full = readMilli(path, "energy_full")
		b.Design, e.Design = readMilli(path, "energy_full_design")
		b.ChargeRate, e.ChargeRate = readMilli(path, "power_now")
	}

	if e.Capacity != nil && e.Current == nil && e.Full != nil {
		b.Capacity = b.Current / (b.Full * 100)
		e.Capacity = nil
	}

	status, err := ioutil.ReadFile(filepath.Join(path, "status"))
	if err == nil {
		status := string(status[:len(status)-1])
		b.State.specific = status
		switch status {
		case "Unknown":
			b.State.Raw = Unknown
		case "Empty":
			b.State.Raw = Empty
		case "Full":
			b.State.Raw = Full
		case "Charging":
			b.State.Raw = Charging
		case "Discharging":
			b.State.Raw = Discharging
		case "Not charging":
			b.State.Raw = Idle
		default:
			b.State.Raw = Undefined
		}
	} else {
		e.State = err
	}

	return b, e
}

func systemGet(idx int) (*Battery, error) {
	bFiles, err := getBatteryFiles()
	if err != nil {
		return nil, err
	}

	if idx >= len(bFiles) {
		return nil, ErrNotFound
	}
	return getByPath(bFiles[idx])
}

func systemGetAll() ([]*Battery, error) {
	bFiles, err := getBatteryFiles()
	if err != nil {
		return nil, err
	}

	batteries := make([]*Battery, len(bFiles))
	errors := make(Errors, len(bFiles))
	for i, bFile := range bFiles {
		battery, err := getByPath(bFile)
		batteries[i] = battery
		errors[i] = err
	}

	return batteries, errors
}
