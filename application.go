package oculus_rating_data

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

func getAppData(filename string) ShareApp {
	bits, err := ioutil.ReadFile(filepath.Join(CACHEDIR, filename))
	checkError(err)
	var sa ShareApp
	err = json.Unmarshal(bits, &sa)
	checkError(err)
	return sa
}

// load oculus share data from json files in the specified directory
func GetAppsData(directory string) []ShareApp {
	f, err := os.Open(directory)
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
