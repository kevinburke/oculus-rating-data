package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"code.google.com/p/go.net/html"
)

const FORCE_DOWNLOAD = false
const HOMEPAGE = "tmp/oculus_share_homepage.html"

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

// Gets all application links from the homepage.
// This function calls itself recursively
func checkElement(n *html.Node) []string {
	links := make([]string, 0)
	for _, elem := range n.Attr {
		if elem.Val == "nameWrap" && elem.Key == "class" {
			childLink := n.FirstChild
			if childLink == nil {
				continue
			}
			for _, linkElem := range childLink.Attr {
				if linkElem.Key == "href" {
					links = append(links, linkElem.Val)
				}
			}
		}
	}
	rLinks := make([]string, 0)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		rLinks = append(rLinks, checkElement(c)...)
		rLinks = append(rLinks, links...)
	}
	return rLinks
}

func getPage(method string, path string) (*http.Response, error) {
	log.Printf("getting %s", path)
	req, err := http.NewRequest(method, "https://share.oculusvr.com"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "twenty-milliseconds-bot/1.0 (go/1.3)")
	client := &http.Client{}
	return client.Do(req)
}

func writePage(page io.Reader, filename string) error {
	log.Printf("writing %s to disk", filename)
	fo, err := os.Create(filename)
	defer fo.Close()
	if err != nil {
		return err
	}
	w := bufio.NewWriter(fo)
	_, err = w.ReadFrom(page)
	if err != nil {
		return err
	}
	checkError(err)
	err = w.Flush()
	return err
}

// get the unique path part for each app
// /app/foobar -> foobar
func getAppId(path string) string {
	if !strings.HasPrefix(path, "/app/") {
		panic("expected path " + path + " to start with /app, didn't")
	}
	return strings.Replace(path, "/app/", "", -1)
}

func _cachePage(path string, fn string) {
	// hack to use the json API instead of the html
	apiPath := strings.Replace(path, "/app/", "/apps-url-map/", -1)
	r, err := getPage("GET", apiPath)
	defer r.Body.Close()
	if err != nil {
		log.Printf("error with path %s: %s\n", path, err.Error())
	}
	err = writePage(r.Body, fn)
	if err != nil {
		log.Printf("error with path %s: %s\n", path, err.Error())
	}
}

func cachePage(path string, directory string, id string, wg *sync.WaitGroup, forceDownload bool) {
	defer wg.Done()
	fn := filepath.Join(directory, id)
	if FORCE_DOWNLOAD {
		_cachePage(path, fn)
	}
	// logic here is a little unwieldy, fuck
	_, ferr := os.Stat(fn)
	if os.IsNotExist(ferr) {
		_cachePage(path, fn)
	}
}

// downloads all ratings and stores them in the tmp directory as json
// files takes about 12 seconds to run. please do not run this with
// forceDownload=true, just use the cache
func fetchEverything(forceDownload bool) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	cachePage("/category/all", "tmp", "share_homepage.html", wg, forceDownload)
	f, err := os.Open(filepath.Join("tmp", "share_homepage.html"))
	checkError(err)
	doc, err := html.Parse(f)
	checkError(err)
	links := checkElement(doc)
	for _, link := range links {
		id := getAppId(link)
		wg.Add(1)
		go cachePage(link, "tmp", id+".json", wg, forceDownload)
	}
	wg.Wait()
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
