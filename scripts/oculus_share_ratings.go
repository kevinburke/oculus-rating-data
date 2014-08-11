package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const FORCE_DOWNLOAD = false

type ComfortLevel uint8

const (
	VeryNauseating ComfortLevel = iota
	Nauseating
	Uncomfortable
	Moderate
	Comfortable
)

type ShareApp struct {
	Name string `json:"name"`
	// Number between 1 and 50
	UserRating   uint32 `json:"rating"`
	Comfort      uint32 `json:"comfortRating"`
	ComfortVotes uint32 `json:"comfortVotes"`
	Ratings      uint32 `json:"votes"`
	Downloads    uint32 `json:"fileDownloadsOculus"`
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err.Error())
	}
}

func getAppData(filename string) ShareApp {
	bits, err := ioutil.ReadFile(filepath.Join("tmp", filename))
	checkError(err)
	var sa ShareApp
	err = json.Unmarshal(bits, &sa)
	checkError(err)
	return sa
}

// load oculus share data from the tmp folder.
func getAppsData() []ShareApp {
	f, err := os.Open("tmp")
	checkError(err)
	defer f.Close()
	names, err := f.Readdirnames(-1)
	checkError(err)
	fmt.Printf("Name, UserRating, Ratings, Comfort, ComfortVotes, Downloads\n")
	sas := make([]ShareApp, 0)
	for _, file := range names {
		if !strings.HasSuffix(file, ".json") {
			continue
		}
		sa := getAppData(file)
		//fmt.Printf("%s, %d, %d, %d, %d, %d\n", sa.Name, sa.UserRating, sa.Ratings, sa.Comfort, sa.ComfortVotes, sa.Downloads)
		sas = append(sas, sa)
	}
	return sas
}

type FRAppRatingOutput struct {
	Names      []string               `json:"names"`
	Data       []FRAppRatingDataPoint `json:"data"`
	Slope      float64                `json:"slope"`
	YIntercept float64                `json:"y_intercept"`
}

type FRAppRatingDataPoint []float32

func getCSVData() [][]string {
	f, err := os.Open("static/csv/dk2_fps_names.csv")
	checkError(err)
	defer f.Close()
	rdr := csv.NewReader(f)
	r, err := rdr.ReadAll()
	checkError(err)
	return r
}

// serialize data from my two sources into the desired struct - frame rate vs
// oculus share rating.
func getFRShareRatingData(sas []ShareApp, records [][]string) (*FRAppRatingOutput, error) {
	frro := new(FRAppRatingOutput)
	// start at 1 to skip header row
	for i := 1; i < len(records); i++ {
		if records[i][1] == "" {
			// no corresponding share app
			continue
		}
		for j := 0; j < len(sas); j++ {
			if sas[j].Name == records[i][1] {
				quot := float32(sas[j].UserRating) / float32(sas[j].Ratings)
				frro.Names = append(frro.Names, sas[j].Name)
				framerate, err := strconv.ParseFloat(records[i][5], 32)
				if err != nil {
					return &FRAppRatingOutput{}, err
				}
				if framerate > 200 {
					fmt.Println("skipping ", sas[j].Name, "due to framerate", framerate)
					break
				}
				dp := []float32{float32(framerate), quot}
				frro.Data = append(frro.Data, dp)
				break
			}
			if j == len(sas)-1 {
				fmt.Println("didn't find ", records[i][1])
			}
		}
	}
	return frro, nil
}

// ratings vs downloads
func getRatingDownloadData(sas []ShareApp) *FRAppRatingOutput {
	frro := new(FRAppRatingOutput)
	for j := 0; j < len(sas); j++ {
		if sas[j].Ratings <= 5 {
			continue
		}
		quot := float32(sas[j].UserRating) / float32(sas[j].Ratings)
		frro.Names = append(frro.Names, sas[j].Name)
		dp := []float32{quot, float32(sas[j].Downloads)}
		frro.Data = append(frro.Data, dp)
	}
	return frro
}

func getComfortLevelRatingData(sas []ShareApp) *FRAppRatingOutput {
	frro := new(FRAppRatingOutput)
	for j := 0; j < len(sas); j++ {
		if sas[j].Ratings <= 5 || sas[j].ComfortVotes <= 5 {
			continue
		}
		quot := float32(sas[j].UserRating) / float32(sas[j].Ratings)
		quot2 := float32(sas[j].Comfort) / float32(sas[j].ComfortVotes)
		frro.Names = append(frro.Names, sas[j].Name)
		dp := []float32{quot2, quot}
		frro.Data = append(frro.Data, dp)
	}
	return frro
}

func getFRComfortLevelData(sas []ShareApp, records [][]string) (*FRAppRatingOutput, error) {
	frro := new(FRAppRatingOutput)
	// start at 1 to skip header row
	for i := 1; i < len(records); i++ {
		if records[i][1] == "" {
			// no corresponding share app
			continue
		}
		for j := 0; j < len(sas); j++ {
			if sas[j].Name == records[i][1] {
				quot := float32(sas[j].Comfort) / float32(sas[j].ComfortVotes)
				frro.Names = append(frro.Names, sas[j].Name)
				framerate, err := strconv.ParseFloat(records[i][5], 32)
				if err != nil {
					return &FRAppRatingOutput{}, err
				}
				if framerate > 200 {
					framerate = 200
				}
				dp := []float32{float32(framerate), quot}
				frro.Data = append(frro.Data, dp)
				break
			}
			if j == len(sas)-1 {
				fmt.Println("didn't find ", records[i][1])
			}
		}
	}
	return frro, nil
}

func dumpFRROToFile(frro *FRAppRatingOutput, filename string) error {
	out, err := json.Marshal(frro)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, out, 0644)
}

// Returns slope/ y intercept of the trend line for the data
func computeSlopeYIntercept(frro *FRAppRatingOutput) (float64, float64) {
	sumX, sumY, sumXX, sumXY := 0.0, 0.0, 0.0, 0.0
	lfl := float64(len(frro.Data))
	for _, d := range frro.Data {
		sumX += float64(d[0])
		sumY += float64(d[1])
		sumXX += float64(d[0]) * float64(d[0])
		sumXY += float64(d[0]) * float64(d[1])
	}
	slope := (lfl*sumXY - sumX*sumY) / (lfl*sumXX - sumX*sumX)
	yintercept := (sumY - slope*sumX) / lfl
	return slope, yintercept
}

func main() {
	sas := getAppsData()
	//rs := getCSVData()
	frro := getComfortLevelRatingData(sas)
	frro.Slope, frro.YIntercept = computeSlopeYIntercept(frro)
	fn := filepath.Join("static", "data", "correlation-framerate",
		"comfort_rating.json")
	err := dumpFRROToFile(frro, fn)
	checkError(err)
	fmt.Println("slope: ", frro.Slope)
	fmt.Println("b: ", frro.YIntercept)
}
