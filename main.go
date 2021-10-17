package main

import (
	"fmt"

	"github.com/ztimes2/surfforecast-go/surfforecast"
)

func main() {
	f, err := surfforecast.New().DailyForecast("Cherating")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Day: %s, %d\n", f.Day.Weekday.String(), f.Day.MonthDay)

	for _, hf := range f.HourlyForecasts {
		fmt.Println()
		fmt.Println("----------------------------------------")

		fmt.Printf("Hour: %d\n", hf.Hour)
		fmt.Printf("Rating (0-10): %d\n", hf.Rating)
		fmt.Printf("Wave energy (kJ): %v\n", hf.WaveEnergyInKiloJoules)
		fmt.Printf("Wind speed (km/h): %v\n", hf.Wind.SpeedInKilometersPerHour)
		fmt.Printf("Wind direction (degrees): %v\n", hf.Wind.DirectionInDegrees)
		fmt.Printf("Wind direction (compass points): %s\n", hf.Wind.DirectionInCompassPoints)
		fmt.Printf("Wind state: %s\n", hf.Wind.State)

		for i, swell := range hf.Swells {
			fmt.Printf("Swell #%d:\n", i+1)
			fmt.Printf("\tPeriod (s): %v\n", swell.PeriodInSeconds)
			fmt.Printf("\tDirection (degrees): %v\n", swell.DirectionInDegrees)
			fmt.Printf("\tDirection (compass points): %s\n", swell.DirectionInCompassPoints)
			fmt.Printf("\tWave height (m): %v\n", swell.WaveHeightInMeters)
		}
	}
}
