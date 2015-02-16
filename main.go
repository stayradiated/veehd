package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mgutz/ansi"
	"golang.org/x/net/html"
)

type Movie struct {
	Title       string
	Url         string
	Description string
	Duration    string
	Size        string
	Posted      string
	ViewCount   string
	Type        string
	Bitrate     string
	Resolution  string
}

func (m Movie) Pretty(id int) string {
	reset := ansi.ColorCode("reset")

	return fmt.Sprintf("%s%d%s: %s%s%s\n%s%s%s\n%s%s - %s - %s - %s -%s %s - %s%s - %s%s\n",
		ansi.ColorCode("red:black"), id, reset,
		ansi.ColorCode("yellow"), m.Title, reset,
		ansi.ColorCode("white+h"), m.Description, reset,
		ansi.ColorCode("blue"), m.Duration, m.Size, m.Posted, m.ViewCount, m.Type, reset,
		ansi.ColorCode("green:black"), m.Bitrate, m.Resolution, reset,
	)
}

func main() {
	query := strings.Join(os.Args[1:], " ")

	Search("\"" + strings.Replace(query, " ", "+", -1) + "\"")
}

func Search(query string) {
	url := "http://veehd.com/search?q=" + query

	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Fatal(err)
	}

	results := make([]*Movie, 0)

	// Loop through each result
	doc.Find("table.movieList > tbody > tr").Each(func(i int, s *goquery.Selection) {
		result := new(Movie)

		result.Title = s.Find("td > h2 > a").Text()
		result.Description = s.Find("td > span:nth-child(4)").Text()

		url, _ := s.Find("td > h2 > a").Attr("href")
		result.Url = "http://veehd.com" + url

		details := s.Find("td > span > span.dr")
		result.Duration = details.Eq(0).Text()
		result.Size = details.Eq(1).Text()
		result.Posted = details.Eq(2).Text()
		result.ViewCount = details.Eq(3).Text()

		results = append(results, result)
	})

	fmt.Printf("Found %d results...\n", len(results))

	for i, result := range results {
		ScrapeMovie(result)
		fmt.Println(result.Pretty(i))
	}

	var i int
	_, err = fmt.Scanf("%d", &i)
	if err != nil {
		log.Fatal(err)
	}

	GetDownloadLink(results[i])
}

var VpiRegex = regexp.MustCompile("\"/vpi?.+do=d.+\"")
var BitrateRegex = regexp.MustCompile("bitrate: (\\d* kb/s)")
var ResolutionRegex = regexp.MustCompile("resolution: (\\d*x\\d*)")
var TypeRegex = regexp.MustCompile("type: (\\w+)")

func ScrapeMovie(m *Movie) {
	doc, err := goquery.NewDocument(m.Url)
	if err != nil {
		log.Fatal(err)
	}

	details := doc.Find(".info > table > tbody > tr > td:nth-child(2) > div").Text()
	m.Bitrate = BitrateRegex.FindStringSubmatch(details)[1]
	m.Resolution = ResolutionRegex.FindStringSubmatch(details)[1]
	m.Type = TypeRegex.FindStringSubmatch(details)[1]

	// http://stackoverflow.com/a/3442757
	desc := doc.Find(".info > table > tbody > tr > td:nth-child(3) > span > div")
	m.Description = strings.TrimSpace(desc.Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
		return s.Get(0).Type == html.TextNode
	}).Text())
}

func GetDownloadLink(m *Movie) {
	doc, err := goquery.NewDocument(m.Url)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("script[type=\"text/javascript\"]").Each(func(i int, s *goquery.Selection) {
		vpiMatch := VpiRegex.FindString(s.Text())

		if len(vpiMatch) > 0 {
			url := "http://veehd.com" + strings.Trim(vpiMatch, "\"")
			link, requireRefresh := HandleVaPage(url)

			if requireRefresh {
				FetchUrl("http://veehd.com" + link)
				link, _ = HandleVaPage(url)
			}

			fmt.Println(link)
		}
	})
}

func HandleVaPage(s string) (link string, requireRefresh bool) {
	doc, err := goquery.NewDocument(s)
	if err != nil {
		log.Fatal(err)
	}

	iframe := doc.Find("iframe")
	if iframe.Length() > 0 {
		link, _ := iframe.Attr("src")
		return link, true
	}

	if doc.Find("h2").Length() > 0 {
		link, _ := doc.Find("a").Attr("href")
		return link, false
	}

	log.Fatal("Could not get download link")
	return "", false
}

func FetchUrl(s string) {
	resp, err := http.Get(s)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
}
