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
		fmt.Printf("Hour: %d\n", hf.Hour)
		fmt.Printf("Rating: %d/10\n", hf.Rating)
		fmt.Printf("Swells: %+v\n", hf.Swells)
	}
}
