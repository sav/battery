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

// +build freebsd dragonfly

package battery

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func readUint32(bytes []byte) uint32 {
	var ret uint32
	for i, b := range bytes {
		ret |= uint32(b) << (uint32(i) * 8)
	}
	return ret
}

func readFloat(bytes []byte) float64 {
	return float64(readUint32(bytes))
}

func ioctl(fd, nr int, retptr *[164]byte) error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		// Some magicks derived from sys/ioccom.h.
		uintptr((0x40000000|0x80000000)|
			((int(unsafe.Sizeof(*retptr))&(1<<13-1))<<16)|
			('B'<<8)|
			nr,
		),
		uintptr(unsafe.Pointer(retptr)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func get(idx int) (*Battery, error) {
	// TODO: Checks for UNKNOWN_CAP
	fd, err := unix.Open("/dev/acpi", unix.O_RDONLY, 0777)
	if err != nil {
		return nil, FatalError{Err: err}
	}
	defer unix.Close(fd)

	b := &Battery{Name: fmt.Sprintf("BAT%d", idx)}
	e := PartialError{}

	// No unions in Go, so lets "emulate" union with byte array ;-].
	var retptr [164]byte
	unit := (*int)(unsafe.Pointer(&retptr[0]))

	*unit = idx
	err = ioctl(fd, 0x10, &retptr)
	if err == nil {
		b.Design = readFloat(retptr[4:8]) // acpi_bif.dcap
		b.Full = readFloat(retptr[8:12])  // acpi_bif.lfcap
	} else {
		e.Design = err
		e.Full = err
	}

	*unit = idx
	err = ioctl(fd, 0x11, &retptr)
	if err == nil {
		var stateString string
		switch readUint32(retptr[0:4]) { // acpi_bst.state
		case 0x0000:
			stateString = "Full"
		case 0x0001:
			stateString = "Discharging"
		case 0x0002:
			stateString = "Charging"
		case 0x0004:
			stateString = "Empty"
		default:
			stateString = "Unknown"
		}
		b.State, _ = newState(stateString)
		b.ChargeRate = readFloat(retptr[4:8]) // acpi_bst.rate
		b.Current = readFloat(retptr[8:12])   // acpi_bst.cap
	} else {
		e.State = err
		e.ChargeRate = err
		e.Current = err
	}

	if !e.Nil() {
		return b, e
	}
	return b, nil
}

// There is no way to iterate over available batteries.
// Therefore we assume here that if we were not able to retrieve
// anything, it means we're done.
func getAll() ([]*Battery, error) {
	var batteries []*Battery
	var errors Errors
	for i := 0; ; i++ {
		b, err := get(i)
		if perr, ok := err.(PartialError); ok && perr.NoNil() {
			break
		}
		batteries = append(batteries, b)
		errors = append(errors, err)
	}

	if errors.Nil() {
		return batteries, nil
	}
	return batteries, errors
}
