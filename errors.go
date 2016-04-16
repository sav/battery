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

var NotFoundError = fmt.Errorf("Not found")

type FatalError struct {
	Err error
}

func (f FatalError) Error() string {
	return fmt.Sprintf("Could not retrieve battery info: `%s`", f.Err)
}

type PartialError struct {
	State      error
	Current    error
	Full       error
	Design     error
	ChargeRate error
}

func (p PartialError) Error() string {
	errors := map[string]error{
		"State":      p.State,
		"Current":    p.Current,
		"Full":       p.Full,
		"Design":     p.Design,
		"ChargeRate": p.ChargeRate,
	}
	s := ""
	for name, err := range errors {
		if err != nil {
			s += fmt.Sprintf("%s: %s\n", name, err.Error())
		}
	}
	return s
}

func (p PartialError) Nil() bool {
	return p.State == nil &&
		p.Current == nil &&
		p.Full == nil &&
		p.Design == nil &&
		p.ChargeRate == nil
}

func (p PartialError) NoNil() bool {
	return p.State != nil &&
		p.Current != nil &&
		p.Full != nil &&
		p.Design != nil &&
		p.ChargeRate != nil
}

type Errors []error

func (e Errors) Error() string {
	s := ""
	for _, err := range e {
		s += err.Error()
	}
	return s
}

func (e Errors) Nil() bool {
	for _, err := range e {
		switch terr := err.(type) {
		case FatalError:
			return false
		case PartialError:
			if !terr.Nil() {
				return false
			}
		default:
			if terr != nil {
				return false
			}
		}
	}
	return true
}
