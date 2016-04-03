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
	"sort"
	"strconv"
	"unsafe"

	"github.com/distatus/go-plist"

	"golang.org/x/sys/unix"
)

type plistref struct {
	pref_plist unsafe.Pointer
	pref_len   uint64
}

type prop []struct {
	Description string `plist:"description"`
	CurValue    int    `plist:"cur-value"`
	MaxValue    int    `plist:"max-value"`
	State       string `plist:"state"`
}

type props map[string]prop

func readBytes(ptr unsafe.Pointer, length uint64) []byte {
	buf := make([]byte, length-1)
	uiptr := uintptr(ptr)
	var i uint64
	for ; i < length-1; i++ {
		buf[i] = *(*byte)(unsafe.Pointer(uiptr))
		uiptr++
	}
	return buf
}

func readProps() (props, error) {
	fd, err := unix.Open("/dev/sysmon", unix.O_RDONLY, 0777)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	var retptr plistref

	if err = ioctl(fd, 0, 'E', unsafe.Sizeof(retptr), unsafe.Pointer(&retptr)); err != nil {
		return nil, err
	}
	bytes := readBytes(retptr.pref_plist, retptr.pref_len)

	var props props
	if _, err = plist.Unmarshal(bytes, &props); err != nil {
		return nil, err
	}
	return props, nil
}

func convertBattery(idx int, prop prop) *Battery {
	battery := &Battery{Name: fmt.Sprintf("BAT%d", idx)}
	var stateGuard int8
	for _, val := range prop {
		switch val.Description {
		case "design cap":
			battery.Design = float64(val.CurValue)
		case "last full cap":
			battery.Full = float64(val.CurValue)
		case "charge":
			battery.Current = float64(val.CurValue)
			if val.CurValue == val.MaxValue {
				battery.State, _ = newState("Full")
			}
		case "charge rate":
			if val.State == "valid" {
				battery.ChargeRate = float64(val.CurValue)
				battery.State, _ = newState("Charging")
				stateGuard++
			}
		case "discharge rate":
			if val.State == "valid" {
				battery.ChargeRate = float64(val.CurValue)
				battery.State, _ = newState("Discharging")
				stateGuard++
			}
		}
	}
	if stateGuard == 2 {
		battery.State, _ = newState("Unknown")
	}
	return battery
}

func get(idx int) (*Battery, error) {
	props, err := readProps()
	if err != nil {
		return nil, FatalError{Err: err}
	}

	for key, vals := range props {
		base, idxStr := key[:7], key[7:]
		if base != "acpibat" || idxStr != strconv.Itoa(idx) {
			continue
		}

		return convertBattery(idx, vals), nil
	}
	return nil, FatalError{Err: fmt.Errorf("Not found")}
}

func getAll() ([]*Battery, error) {
	props, err := readProps()
	if err != nil {
		return nil, FatalError{Err: err}
	}

	var keys []string
	for key := range props {
		if key[:7] != "acpibat" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	batteries := make([]*Battery, len(keys))
	errors := make(Errors, len(keys))
	for i, key := range keys {
		idx, err := strconv.Atoi(key[7:])
		if err != nil {
			// Malformed battery entry name.
			// This should not happen in reality.
			errors[i] = FatalError{Err: err}
			continue
		}

		batteries[i] = convertBattery(idx, props[key])
	}

	if errors.Nil() {
		return batteries, nil
	}
	return batteries, errors
}
