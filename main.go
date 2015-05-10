package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
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
	Bitrate     int
	ResolutionX int
	ResolutionY int
}

func (m Movie) ResolutionArea() int {
	return m.ResolutionX * m.ResolutionY
}

func (m Movie) Pretty(id int) string {
	reset := ansi.ColorCode("reset")

	return fmt.Sprintf("%s%d%s: %s%s%s\n%s%s%s\n%s%s - %s - %s - %s -%s %s - %s%d kb/s - %dx%d%s\n",
		ansi.ColorCode("red:black"), id, reset,
		ansi.ColorCode("yellow"), m.Title, reset,
		ansi.ColorCode("white+h"), m.Description, reset,
		ansi.ColorCode("blue"), m.Duration, m.Size, m.Posted, m.ViewCount, m.Type, reset,
		ansi.ColorCode("green:black"), m.Bitrate, m.ResolutionX, m.ResolutionY, reset,
	)
}

type ByQuality []*Movie

func (a ByQuality) Len() int      { return len(a) }
func (a ByQuality) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByQuality) Less(i, j int) bool {
	if a[i].ResolutionArea() == a[j].ResolutionArea() {
		return a[i].Bitrate < a[j].Bitrate
	}
	return a[i].ResolutionArea() < a[j].ResolutionArea()
}

func main() {

	var searchTerm string
	var atIndex int
	var sortByQuality bool
	var wrapQuotes bool

	var VeehdCmd = &cobra.Command{
		Use:   "veehd",
		Short: "a command line interface for veehd.com",
		Run: func(cmd *cobra.Command, args []string) {
			searchTerm = strings.Replace(searchTerm, " ", "+", -1)
			if wrapQuotes {
				searchTerm = "\"" + searchTerm + "\""
			}
			Search(searchTerm, atIndex, sortByQuality)
		},
	}

	VeehdCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("veehd v0.1 -- HEAD")
		},
	})

	VeehdCmd.Flags().StringVarP(&searchTerm, "search", "s", "", "Term to search for")
	VeehdCmd.Flags().IntVarP(&atIndex, "get", "g", -1, "Don't display results, just return url for selected video")
	VeehdCmd.Flags().BoolVarP(&sortByQuality, "sort", "t", false, "Sort movies by quality (low to high)")
	VeehdCmd.Flags().BoolVar(&wrapQuotes, "quotes", true, "Wrap search term in quotes")
	VeehdCmd.Execute()
}

func Search(query string, index int, sortByQuality bool) {
	url := "http://veehd.com/search?q=" + query

	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Fatal(err)
	}

	results := make([]*Movie, 0)

	// Loop through each result
	doc.Find("table.movieList > tbody > tr").Each(func(i int, s *goquery.Selection) {
		if s.Find(".error_message").Length() > 0 {
			return
		}

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

	if len(results) <= 0 {
		log.Fatal("No results found for", query)
		return
	}

	if index > -1 {
		if index >= len(results) {
			log.Fatal("Invalid index")
		}

		GetDownloadLink(results[index])
		return
	}

	fmt.Printf("Found %d results...\n", len(results))

	if sortByQuality {
		for _, result := range results {
			ScrapeMovie(result)
		}
		sort.Sort(ByQuality(results))
		for i, result := range results {
			fmt.Println(result.Pretty(i))
		}
	} else {
		for i, result := range results {
			ScrapeMovie(result)
			fmt.Println(result.Pretty(i))
		}
	}

	fmt.Println("Select movie: ")

	var i int
	_, err = fmt.Scanf("%d", &i)
	if err != nil {
		log.Fatal(err)
	}

	GetDownloadLink(results[i])
}

var VpiRegex = regexp.MustCompile("\"/vpi?.+do=d.+\"")
var BitrateRegex = regexp.MustCompile("bitrate: (\\d*) kb/s")
var ResolutionRegex = regexp.MustCompile("resolution: (\\d*)x(\\d*)")
var TypeRegex = regexp.MustCompile("type: (\\w+)")

func ScrapeMovie(m *Movie) {
	doc, err := goquery.NewDocument(m.Url)
	if err != nil {
		log.Fatal(err)
	}

	details := doc.Find(".info > table > tbody > tr > td:nth-child(2) > div").Text()
	m.Type = TypeRegex.FindStringSubmatch(details)[1]

	var bitrate string = BitrateRegex.FindStringSubmatch(details)[1]
	m.Bitrate, err = strconv.Atoi(bitrate)
	if err != nil {
		log.Fatal(err)
	}

	var resolution []string = ResolutionRegex.FindStringSubmatch(details)
	m.ResolutionX, _ = strconv.Atoi(resolution[1])
	m.ResolutionY, _ = strconv.Atoi(resolution[2])

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
