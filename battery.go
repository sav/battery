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
