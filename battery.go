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

import "fmt"

type State int

const (
	Empty State = iota
	Full
	Charging
	Discharging
	Unknown
)

var states = [...]string{"Empty", "Full", "Charging", "Discharging", "Unknown"}

func (s State) String() string {
	return states[s]
}

func newState(name string) (State, error) {
	for i, state := range states {
		if name == state {
			return State(i), nil
		}
	}
	return Unknown, fmt.Errorf("Invalid state `%s`", name)
}

type Battery struct {
	Name       string
	State      State
	Current    float64
	Full       float64
	Design     float64
	ChargeRate float64
}

func Get(idx int) (*Battery, error) {
	return get(idx)
}

func GetAll() ([]*Battery, error) {
	return getAll()
}
