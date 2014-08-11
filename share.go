package oculus_rating_data

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"code.google.com/p/go.net/html"
)

const CACHEDIR = "cache"

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

// get the unique path part for each app
// /app/foobar -> foobar
func getAppId(path string) string {
	if !strings.HasPrefix(path, "/app/") {
		panic("expected path " + path + " to start with /app, didn't")
	}
	return strings.Replace(path, "/app/", "", -1)
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

// downloads all ratings and stores them in the cache directory as json
// files takes about 12 seconds to run. please do not run this with
// forceDownload=true, just use the cache
func FetchEverything(forceDownload bool) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	cachePage("/category/all", CACHEDIR, "share_homepage.html", wg, forceDownload)
	f, err := os.Open(filepath.Join(CACHEDIR, "share_homepage.html"))
	checkError(err)
	doc, err := html.Parse(f)
	checkError(err)
	links := checkElement(doc)
	for _, link := range links {
		id := getAppId(link)
		wg.Add(1)
		go cachePage(link, CACHEDIR, id+".json", wg, forceDownload)
	}
	wg.Wait()
}
