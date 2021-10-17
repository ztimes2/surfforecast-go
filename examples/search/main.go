package main

import (
	"fmt"

	"github.com/ztimes2/surfforecast-go"
)

func main() {
	results, err := surfforecast.New().SearchBreaks("che")
	if err != nil {
		panic(err)
	}

	fmt.Println(results)
}
