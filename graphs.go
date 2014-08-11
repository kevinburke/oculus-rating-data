package oculus_rating_data

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
)

type FRAppRatingOutput struct {
	Names      []string               `json:"names"`
	Data       []FRAppRatingDataPoint `json:"data"`
	Slope      float64                `json:"slope"`
	YIntercept float64                `json:"y_intercept"`
}

type FRAppRatingDataPoint []float32

// Returns slope/ y intercept of the trend line for the data
func ComputeSlopeYIntercept(frro *FRAppRatingOutput) (float64, float64) {
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

// serialize data from my two sources into the desired struct - frame rate vs
// oculus share rating.
func GetFRShareRatingData(sas []ShareApp, records [][]string) (*FRAppRatingOutput, error) {
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
func GetRatingDownloadData(sas []ShareApp) *FRAppRatingOutput {
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

func GetComfortLevelRatingData(sas []ShareApp) *FRAppRatingOutput {
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

func GetFRComfortLevelData(sas []ShareApp, records [][]string) (*FRAppRatingOutput, error) {
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

func DumpFRROToFile(frro *FRAppRatingOutput, filename string) error {
	out, err := json.Marshal(frro)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, out, 0644)
}
