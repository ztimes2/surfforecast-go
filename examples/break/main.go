package main

import (
	"fmt"

	"github.com/ztimes2/surfforecast-go"
)

func main() {
	b, err := surfforecast.New().Break("Banzai-Pipelines-and-Backdoor")
	if err != nil {
		panic(err)
	}

	fmt.Println(b)
}
