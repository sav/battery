package battery

import "fmt"

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
