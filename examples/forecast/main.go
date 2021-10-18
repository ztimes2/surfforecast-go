package main

import (
	"fmt"

	"github.com/ztimes2/surfforecast-go"
)

func main() {
	ff, err := surfforecast.New().ForecastsForEightDays("cherating")
	if err != nil {
		panic(err)
	}

	fmt.Println("Issued at:", ff.IssuedAt)

	f := ff.Daily[0]

	fmt.Printf("Date: %s\n", f.Timestamp)

	for _, hf := range f.Hourly {
		fmt.Println()
		fmt.Println("----------------------------------------")

		fmt.Printf("Date: %s\n", hf.Timestamp)
		fmt.Printf("Rating (0-10): %d\n", hf.Rating)
		fmt.Printf("Wave energy (kJ): %v\n", hf.WaveEnergyInKiloJoules)
		fmt.Printf("Wind speed (km/h): %v\n", hf.Wind.SpeedInKilometersPerHour)
		fmt.Printf("Wind direction to (degrees): %v\n", hf.Wind.DirectionToInDegrees)
		fmt.Printf("Wind direction from (compass points): %s\n", hf.Wind.DirectionFromInCompassPoints)
		fmt.Printf("Wind state: %s\n", hf.Wind.State)

		for i, swell := range hf.Swells {
			fmt.Printf("Swell #%d:\n", i+1)
			fmt.Printf("\tPeriod (s): %v\n", swell.PeriodInSeconds)
			fmt.Printf("\tDirection to (degrees): %v\n", swell.DirectionToInDegrees)
			fmt.Printf("\tDirection from (compass points): %s\n", swell.DirectionFromInCompassPoints)
			fmt.Printf("\tWave height (m): %v\n", swell.WaveHeightInMeters)
		}
	}
}
