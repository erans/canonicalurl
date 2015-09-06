package canonicalurl

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"golang.org/x/net/publicsuffix"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"

	"github.com/PuerkitoBio/goquery"
)

type canonicalURLElement struct {
	Name      string
	Selector  string
	Attribute string
}

var canonicalURLLocations = []canonicalURLElement{
	{"canonical", "link[rel='canonical']", "href"},
	{"opengraph", "meta[property='og:url']", "content"},
}

var servicesRequiresVPN = []string{
	"nytimes.com",
}

type resultData struct {
	Result string `json:"result"`
	URL    string `json:"url"`
}

func normalizeURL(url string) string {
	// Remove the protocol part to gain 7-8 more characters
	url = strings.Replace(url, "https://", "", -1)
	url = strings.Replace(url, "http://", "", -1)
	return url
}

func findElementInDocument(doc *goquery.Document, selector string, attribute string) string {
	var value = ""
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr(attribute); exists {
			value = val
		}
	})

	return value
}

var tpl = template.Must(template.ParseGlob("templates/*.html"))

func init() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/get", canonicalURLHandler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if err := tpl.ExecuteTemplate(w, "index.html", nil); err != nil {

	}
}

func canonicalURLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	url := r.URL.Query().Get("url")

	types := r.URL.Query().Get("types")

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	if url == "" {
		http.Error(w, "Missing 'url'", http.StatusBadRequest)
		return
	}

	// Use a big enough hash for URLs over 240 characters (max key in appengine memcache is 250)
	// TODO: Consider doing some minor local canonicalization such as lower case the protocol://host:port/ part before hashing
	var key = normalizeURL(url)
	if len(key) > 240 {
		hash := sha512.New()
		hash.Write([]byte(url))
		urlHash := hash.Sum(nil)
		key = hex.EncodeToString(urlHash)
	}

	callback := r.URL.Query().Get("callback")

	c := appengine.NewContext(r)

	var canonicalURL = ""
	var resultString = ""

	if item, err := memcache.Get(c, key); err == memcache.ErrCacheMiss {
		options := cookiejar.Options{
			PublicSuffixList: publicsuffix.List,
		}

		jar, err := cookiejar.New(&options)
		if err != nil {
			log.Fatal(err)
		}

		client := urlfetch.Client(c)
		// Set a cookie jar to handle sites that tries to block crawlers by
		// setting a cookie like the NY Times.
		client.Jar = jar

		resp, err := client.Get(url)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if resp.StatusCode == http.StatusSeeOther {

		}

		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var locations []canonicalURLElement
		if types != "" {
			requestedTypes := strings.Split(strings.Trim(types, " "), ",")
			locations = make([]canonicalURLElement, len(requestedTypes))
			for i := 0; i < len(requestedTypes); i++ {
				for j := 0; j < len(canonicalURLLocations); j++ {
					if requestedTypes[i] == canonicalURLLocations[j].Name {
						locations[i] = canonicalURLLocations[j]
					}
				}
			}
		} else {
			locations = canonicalURLLocations
		}

		log.Println(locations)
		for i := 0; i < len(locations); i++ {
			canonicalURL = findElementInDocument(doc, locations[i].Selector, locations[i].Attribute)
			if canonicalURL != "" {
				break
			}
		}

		result := &resultData{Result: "ok", URL: canonicalURL}
		resultValue, _ := json.Marshal(result)
		resultString = fmt.Sprintf("%s", resultValue)

		item := &memcache.Item{
			Key:   key,
			Value: []byte(resultString),
		}

		if err := memcache.Add(c, item); err == memcache.ErrNotStored {
			log.Println("Failed to add. Already exists")
		} else {
			log.Println("Added to cache")
		}
	} else {
		log.Println("from cache")
		resultString = fmt.Sprintf("%s", item.Value)
	}

	w.Header().Set("Content-Type", "application/json")
	if callback != "" {
		fmt.Fprintf(w, "/**/ %s(%s)", callback, resultString)
	} else {
		if callback == "" && format == "text" {
			w.Header().Set("Content-Type", "text/plain")
			temp := &resultData{Result: "ok", URL: ""}
			json.Unmarshal([]byte(resultString), &temp)
			fmt.Fprintf(w, "%s", temp.URL)
		} else {
			fmt.Fprintf(w, "%s", resultString)
		}
	}
}
