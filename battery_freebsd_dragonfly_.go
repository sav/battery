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
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func readUint32(bytes []byte) uint32 {
	var ret uint32
	for i, b := range bytes {
		ret |= uint32(b) << uint(i*8)
	}
	return ret
}

func uint32ToFloat64(num uint32) (float64, error) {
	if num == 0xffffffff {
		return 0, errors.New("Unknown value received")
	}
	return float64(num), nil
}

func ioctl_(fd int, nr int64, retptr *[164]byte) error {
	return ioctl(fd, nr, 'B', unsafe.Sizeof(*retptr), unsafe.Pointer(retptr))
}

func systemGet(idx int) (*Battery, error) {
	fd, err := unix.Open("/dev/acpi", unix.O_RDONLY, 0777)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	b := &Battery{}
	e := ErrPartial{}
	var mw bool

	// No unions in Go, so lets "emulate" union with byte array ;-].
	var retptr [164]byte
	unit := (*int)(unsafe.Pointer(&retptr[0]))

	*unit = idx
	err = ioctl_(fd, 0x10, &retptr) // ACPIIO_BATT_GET_BIF
	if err != nil {
		return nil, err
	}
	mw = readUint32(retptr[0:4]) == 0 // acpi_bif.units

	b.Design, e.Design = uint32ToFloat64(readUint32(retptr[4:8])) // acpi_bif.dcap
	b.Full, e.Full = uint32ToFloat64(readUint32(retptr[8:12]))    // acpi_bif.lfcap
	if !mw {
		volts, err := uint32ToFloat64(readUint32(retptr[16:20])) // acpi_bif.dvol

		if err == nil {
			b.Design *= volts
			b.Full *= volts
		} else {
			e.Design = err
			e.Full = err
		}
	}

	*unit = idx
	err = ioctl_(fd, 0x11, &retptr) // APCIIO_BATT_GET_BST
	if err == nil {
		switch readUint32(retptr[0:4]) { // acpi_bst.state
		case 0x0000:
			b.State, _ = newState("Full")
		case 0x0001:
			b.State, _ = newState("Discharging")
		case 0x0002:
			b.State, _ = newState("Charging")
		case 0x0004:
			b.State, _ = newState("Empty")
		default:
			b.State, _ = newState("Unknown")
		}
		b.ChargeRate, e.ChargeRate = uint32ToFloat64(readUint32(retptr[4:8])) // acpi_bst.rate
		b.Current, e.Current = uint32ToFloat64(readUint32(retptr[8:12]))      // acpi_bst.cap

		if !mw {
			volts, err := uint32ToFloat64(readUint32(retptr[12:16])) // acpi_bst.volt
			if err == nil {
				b.ChargeRate *= volts
				b.Current *= volts
			} else {
				e.ChargeRate = err
				e.Current = err
			}
		}
	} else {
		e.State = err
		e.ChargeRate = err
		e.Current = err
	}

	return b, e
}

// There is no way to iterate over available batteries.
// Therefore we assume here that if we were not able to retrieve
// anything, it means we're done.
func systemGetAll() ([]*Battery, error) {
	var batteries []*Battery
	var errors Errors
	for i := 0; ; i++ {
		b, err := systemGet(i)
		if perr, ok := err.(ErrPartial); ok && perr.noNil() {
			break
		}
		if errno, ok := err.(syscall.Errno); ok && errno == 6 {
			break
		}
		batteries = append(batteries, b)
		errors = append(errors, err)
	}

	return batteries, errors
}
