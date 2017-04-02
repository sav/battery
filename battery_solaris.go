// battery
// Copyright (C) 2016-2017 Karol 'Kenji Takahashi' Woźniak
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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
)

var errValueNotFound = fmt.Errorf("Value not found")

func readFloat(val string) (float64, error) {
	num, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err
	}
	if num == math.MaxUint32 {
		return 0, fmt.Errorf("Unknown value received")
	}
	return num, nil
}

func readVoltage(val string) (float64, error) {
	voltage, err := readFloat(val)
	if err != nil {
		return 0, err
	}
	return voltage / 1000, nil
}

type errParse int

func (p errParse) Error() string {
	return fmt.Sprintf("Parse error: `%d`", p)
}

type batteryReader struct {
	cmdout *bufio.Scanner
	li     int
	lline  []byte
}

func (r *batteryReader) next() (*Battery, error) {
	b := &Battery{}
	e := ErrPartial{
		Design:        errValueNotFound,
		Full:          errValueNotFound,
		Current:       errValueNotFound,
		ChargeRate:    errValueNotFound,
		State:         errValueNotFound,
		Voltage:       errValueNotFound,
		DesignVoltage: errValueNotFound,
	}
	setErrParse := func(n int) {
		if e.Design == errValueNotFound {
			e.Design = errParse(n)
		}
		if e.Full == errValueNotFound {
			e.Full = errParse(n)
		}
		if e.Current == errValueNotFound {
			e.Current = errParse(n)
		}
		if e.ChargeRate == errValueNotFound {
			e.ChargeRate = errParse(n)
		}
		if e.State == errValueNotFound {
			e.State = errParse(n)
		}
		if e.Voltage == errValueNotFound {
			e.Voltage = errParse(n)
		}
		if e.DesignVoltage == errValueNotFound {
			e.DesignVoltage = errParse(n)
		}
	}

	var exists, amps bool

	for r.cmdout.Scan() {
		exists = true

		var piece []byte
		if r.lline != nil {
			piece = r.lline
			r.lline = nil
		} else {
			pieces := bytes.Split(r.cmdout.Bytes(), []byte{':'})
			if len(pieces) < 4 {
				setErrParse(4)
				continue
			}

			i, err := strconv.Atoi(string(pieces[1]))
			if err != nil {
				setErrParse(1)
				continue
			}

			if i != r.li {
				r.li = i
				r.lline = pieces[3]
				break
			}

			piece = pieces[3]
		}

		values := bytes.Split(piece, []byte{'\t'})
		if len(values) < 2 {
			setErrParse(2)
			continue
		}
		name, value := string(values[0]), string(values[1])

		switch name {
		case "bif_design_cap":
			b.Design, e.Design = readFloat(value)
		case "bif_last_cap":
			b.Full, e.Full = readFloat(value)
		case "bif_unit":
			amps = value != "0"
		case "bif_voltage":
			b.DesignVoltage, e.DesignVoltage = readVoltage(value)
		case "bst_voltage":
			b.Voltage, e.Voltage = readVoltage(value)
		case "bst_rem_cap":
			b.Current, e.Current = readFloat(value)
		case "bst_rate":
			b.ChargeRate, e.ChargeRate = readFloat(value)
		case "bst_state":
			state, err := strconv.Atoi(value)
			if err != nil {
				e.State = err
				continue
			}

			switch {
			case state&1 != 0:
				b.State, e.State = newState("Discharging")
			case state&2 != 0:
				b.State, e.State = newState("Charging")
			case state&4 != 0:
				b.State, e.State = newState("Empty")
			default:
				e.State = fmt.Errorf("Invalid state flag retrieved: `%d`", state)
			}
		}
	}

	if !exists {
		return nil, io.EOF
	}

	if e.DesignVoltage != nil && e.Voltage == nil {
		b.DesignVoltage, e.DesignVoltage = b.Voltage, nil
	}

	if amps {
		if e.DesignVoltage == nil {
			b.Design *= b.DesignVoltage
		} else {
			e.Design = e.DesignVoltage
		}
		if e.Voltage == nil {
			b.Full *= b.Voltage
			b.Current *= b.Voltage
			b.ChargeRate *= b.Voltage
		} else {
			e.Full = e.Voltage
			e.Current = e.Voltage
			e.ChargeRate = e.Voltage
		}
	}

	if b.State == Unknown && e.Current == nil && e.Full == nil && b.Current >= b.Full {
		b.State, e.State = newState("Full")
	}

	return b, e
}

func newBatteryReader() (*batteryReader, error) {
	out, err := exec.Command("kstat", "-p", "-m", "acpi_drv", "-n", "battery B*").Output()
	if err != nil {
		return nil, err
	}

	return &batteryReader{cmdout: bufio.NewScanner(bytes.NewReader(out))}, nil
}

func systemGet(idx int) (*Battery, error) {
	br, err := newBatteryReader()
	if err != nil {
		return nil, err
	}

	return br.next()
}

func systemGetAll() ([]*Battery, error) {
	br, err := newBatteryReader()
	if err != nil {
		return nil, err
	}

	var batteries []*Battery
	var errors Errors
	for b, e := br.next(); e != io.EOF; b, e = br.next() {
		batteries = append(batteries, b)
		errors = append(errors, e)
	}

	return batteries, errors
}
