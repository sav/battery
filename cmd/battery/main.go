package main

import (
	"fmt"
	"time"

	"github.com/distatus/battery"
)

func printBattery(bat *battery.Battery) {
	fmt.Printf(
		"%s: %s, %.2f%%",
		bat.Name,
		bat.State,
		bat.Current/bat.Full*100,
	)
	defer fmt.Println()

	var str string
	var timeNum float64
	switch bat.State {
	case battery.Discharging:
		if bat.ChargeRate == 0 {
			fmt.Print(", discharging at zero rate - will never fully discharge")
			return
		}
		str = "remaining"
		timeNum = bat.Current / bat.ChargeRate
	case battery.Charging:
		if bat.ChargeRate == 0 {
			fmt.Print(", charging at zero rate - will never fully charge")
			return
		}
		str = "until charged"
		timeNum = (bat.Full - bat.Current) / bat.ChargeRate
	default:
		return
	}
	duration, _ := time.ParseDuration(fmt.Sprintf("%fh", timeNum))
	fmt.Printf(", %s %s", duration, str)
}

func main() {
	batteries, err := battery.GetAll()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, bat := range batteries {
		printBattery(bat)
	}
}
