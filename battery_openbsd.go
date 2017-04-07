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
	"bytes"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var errValueNotFound = fmt.Errorf("Value not found")

var sensorW = [4]int32{
	2,  // SENSOR_VOLTS_DC (uV)
	5,  // SENSOR_WATTS (uW)
	7,  // SENSOR_WATTHOUR (uWh)
	10, // SENSOR_INTEGER
}

const (
	sensorA  = 6 // SENSOR_AMPS (uA)
	sensorAH = 8 // SENSOR_AMPHOUR (uAh)
)

type sensordev struct {
	num           int32
	xname         [16]byte
	maxnumt       [21]int32
	sensors_count int32
}

type sensorStatus int32

const (
	unspecified sensorStatus = iota
	ok
	warning
	critical
	unknown
)

type sensor struct {
	desc   [32]byte
	tv     [16]byte // struct timeval
	value  int64
	typ    [4]byte // enum sensor_type
	status sensorStatus
	numt   int32
	flags  int32
}

func sysctl(mib []int32, out unsafe.Pointer, n uintptr) syscall.Errno {
	_, _, e := unix.Syscall6(
		unix.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(out),
		uintptr(unsafe.Pointer(&n)),
		uintptr(unsafe.Pointer(nil)),
		0,
	)
	return e
}

func ampToWatt(err error, val int64, volts float64) (float64, error) {
	if err == errValueNotFound {
		return (float64(val) / 1000) * volts, nil
	}
	return 0, err
}

func sensordevIter(cb func(sd sensordev, i int, err error) bool) {
	mib := []int32{6, 11, 0}
	var sd sensordev
	var idx int
	var i int32
	for i = 0; ; i++ {
		mib[2] = i

		e := sysctl(mib, unsafe.Pointer(&sd), unsafe.Sizeof(sd))
		if e != 0 {
			if e == unix.ENXIO {
				continue
			}
			if e == unix.ENOENT {
				break
			}
		}

		if bytes.HasPrefix(sd.xname[:], []byte("acpibat")) {
			var err error
			if e != 0 {
				err = e
			}
			if cb(sd, idx, err) {
				return
			}
			idx++
		}
	}
}

func getBattery(sd sensordev) (*Battery, error) {
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

	var i int32
	var s sensor
	mib := []int32{6, 11, sd.num, 0, 0}
	for _, w := range sensorW {
		mib[3] = w

		for i = 0; i < sd.maxnumt[w]; i++ {
			mib[4] = i

			if err := sysctl(mib, unsafe.Pointer(&s), unsafe.Sizeof(s)); err != 0 {
				if e.Design == errValueNotFound {
					e.Design = err
				}
				if e.Full == errValueNotFound {
					e.Full = err
				}
				if e.Current == errValueNotFound {
					e.Current = err
				}
				if e.ChargeRate == errValueNotFound {
					e.ChargeRate = err
				}
				if e.State == errValueNotFound {
					e.State = err
				}
				if e.Voltage == errValueNotFound {
					e.Voltage = err
				}
				if e.DesignVoltage == errValueNotFound {
					e.DesignVoltage = err
				}
				continue
			}

			desc := string(s.desc[:bytes.IndexByte(s.desc[:], 0)])

			if strings.HasPrefix(desc, "battery ") {
				//TODO:battery idle(?)
				if desc == "battery critical" {
					b.State, e.State = newState("Empty")
				} else {
					b.State, e.State = newState(desc[8:])
				}
				continue
			}

			switch desc {
			case "rate":
				b.ChargeRate, e.ChargeRate = float64(s.value)/1000, nil
			case "design capacity":
				b.Design, e.Design = float64(s.value)/1000, nil
			case "last full capacity":
				b.Full, e.Full = float64(s.value)/1000, nil
			case "remaining capacity":
				b.Current, e.Current = float64(s.value)/1000, nil
			case "current voltage":
				b.Voltage, e.Voltage = float64(s.value)/1000000, nil
			case "voltage":
				if s.status == unknown {
					e.DesignVoltage = fmt.Errorf("Unknown value received")
					continue
				}
				b.DesignVoltage, e.DesignVoltage = float64(s.value)/1000000, nil
			}
		}
	}

	if e.DesignVoltage != nil && e.Voltage == nil {
		b.DesignVoltage, e.DesignVoltage = b.Voltage, nil
	}

	if e.ChargeRate == errValueNotFound {
		if e.Voltage == nil {
			mib[3] = sensorA

			for i = 0; i < sd.maxnumt[sensorA]; i++ {
				mib[4] = i

				if err := sysctl(mib, unsafe.Pointer(&s), unsafe.Sizeof(s)); err != 0 {
					e.ChargeRate = err
				}

				desc := string(s.desc[:bytes.IndexByte(s.desc[:], 0)])

				if desc != "rate" {
					continue
				}

				b.ChargeRate, e.ChargeRate = (float64(s.value)/1000)*b.Voltage, nil
			}
		} else {
			e.ChargeRate = e.Voltage
		}
	}
	if e.Design == errValueNotFound || e.Full == errValueNotFound || e.Current == errValueNotFound {
		mib[3] = sensorAH

		for i = 0; i < sd.maxnumt[sensorAH]; i++ {
			mib[4] = i

			if err := sysctl(mib, unsafe.Pointer(&s), unsafe.Sizeof(s)); err != 0 {
				// At this point all values are either retrieved or have error set,
				// no need to set error(s) again.
				continue
			}

			desc := string(s.desc[:bytes.IndexByte(s.desc[:], 0)])

			switch desc {
			case "design capacity":
				b.Design, e.Design = ampToWatt(e.Design, s.value, b.DesignVoltage)
			case "last full capacity":
				b.Full, e.Full = ampToWatt(e.Full, s.value, b.Voltage)
			case "remaining capacity":
				b.Current, e.Current = ampToWatt(e.Current, s.value, b.Voltage)
			}
		}
	}

	return b, e
}

func systemGet(idx int) (*Battery, error) {
	var b *Battery
	var e error

	sensordevIter(func(sd sensordev, i int, err error) bool {
		if i == idx {
			if err == nil {
				b, e = getBattery(sd)
			} else {
				e = err
			}
			return true
		}
		return false
	})

	if b == nil {
		return nil, ErrNotFound
	}
	return b, e
}

func systemGetAll() ([]*Battery, error) {
	var batteries []*Battery
	var errors Errors

	sensordevIter(func(sd sensordev, i int, err error) bool {
		var b *Battery
		if err == nil {
			b, err = getBattery(sd)
		}

		batteries = append(batteries, b)
		errors = append(errors, err)
		return false
	})

	return batteries, errors
}
