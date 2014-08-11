package oculus_rating_data

import (
	"encoding/csv"
	"os"
)

// load FPS data from the attached CSV and return it as a 2D array of strings
func GetFPSData(filename string) [][]string {
	f, err := os.Open(filename)
	checkError(err)
	defer f.Close()
	rdr := csv.NewReader(f)
	r, err := rdr.ReadAll()
	checkError(err)
	return r
}
