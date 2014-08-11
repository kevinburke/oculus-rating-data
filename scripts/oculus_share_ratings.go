package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/kevinburke/oculus-rating-data"
)

const CACHEDIR = "cache"

type ComfortLevel uint8

const (
	VeryNauseating ComfortLevel = iota
	Nauseating
	Uncomfortable
	Moderate
	Comfortable
)

func checkError(err error) {
	if err != nil {
		log.Fatal(err.Error())
	}
}

func main() {
	oculus_rating_data.FetchEverything(false)

	sas := oculus_rating_data.GetAppsData("cache")
	//rs := oculus_rating_data.GetCSVData("csv/dk2_fps_names")
	frro := oculus_rating_data.GetComfortLevelRatingData(sas)
	frro.Slope, frro.YIntercept = oculus_rating_data.ComputeSlopeYIntercept(frro)
	fn := filepath.Join("graphs", "comfort_rating.json")
	err := oculus_rating_data.DumpFRROToFile(frro, fn)
	checkError(err)
	fmt.Println("slope: ", frro.Slope)
	fmt.Println("b: ", frro.YIntercept)
}
