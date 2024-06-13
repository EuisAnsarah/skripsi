package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/RadhiFadlillah/go-sastrawi"
)

var (
	stemmer   = sastrawi.NewStemmer(sastrawi.DefaultDictionary())
	stopwords = sastrawi.DefaultStopword()
)

func main() {
	// Create files
	f1, err := os.Create("before.csv")
	if err != nil {
		panic(err)
	}
	defer f1.Close()

	f2, err := os.Create("after.csv")
	if err != nil {
		panic(err)
	}
	defer f2.Close()

	// Write CSV
	writer1 := csv.NewWriter(f1)
	defer writer1.Flush()

	writer2 := csv.NewWriter(f2)
	defer writer2.Flush()

	// Write Header
	writer1.Write([]string{"Title", "Body", "Link"})
	writer2.Write([]string{"Title", "Body", "Link"})

	// Control concurrent fetches
	concurrency := 15
	semaphore := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Fetch pages concurrently
	for page := 1; page <= 600; page++ {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(page int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			var oris, edits []News
			var errCount int

			for errCount < 3 {
				ori, edit, err := getPage(page)
				if err != nil {
					log.Printf("Error fetching page %d: %v\n", page, err)
					errCount++
					time.Sleep(1 * time.Second) // Wait before retrying
					continue
				}
				oris, edits = ori, edit
				break
			}

			for j := 0; j < len(oris); j++ {
				mutex.Lock()
				writer1.Write([]string{oris[j].Title, oris[j].Body, oris[j].Link})
				writer2.Write([]string{edits[j].Title, edits[j].Body, edits[j].Link})
				mutex.Unlock()
			}
		}(page)
	}

	wg.Wait()
}

func getPage(page int) ([]News, []News, error) {
	var oris, edits []News
	var links []string

	// Retry logic
	for retry := 0; retry < 3; retry++ {
		res, err := http.Get("https://www.detik.com/search/searchall?query=banjir&siteid=2&sortby=time&fromdatex=01/01/2023&todatex=30/12/2023&page=" + fmt.Sprint(page))
		if err != nil {
			return nil, nil, err
		}

		if res.StatusCode == http.StatusNotFound {
			log.Printf("Page %d not found. Retrying...\n", page)
			time.Sleep(1 * time.Second) // Wait before retrying
			continue
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
		}

		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			return nil, nil, err
		}

		doc.Find("body > div.wrapper.full > div > div.list.media_rows.list-berita").
			Each(func(i int, s *goquery.Selection) {
				s.Find("a").Each(func(i int, s *goquery.Selection) {
					link := s.AttrOr("href", "")
					links = append(links, link)
				})
			})

		break
	}

	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Fetch news concurrently
	for _, link := range links {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			ori, edit, err := getNews(link)
			if err != nil {
				log.Printf("Error fetching news: %v\n", err)
				return
			}
			mutex.Lock()
			oris = append(oris, ori)
			edits = append(edits, edit)
			mutex.Unlock()
		}(link)
	}

	wg.Wait()
	return oris, edits, nil
}

var titleSelectors = []string{
	"body > div.container > div.grid-row.content__bg > div.column-8 > article > div.detail__header > h1",
	"#content > div.container.detail_content.group > div.l_content > div.group > article > div.jdl > h1",
}

var bodySelectors = []string{
	"body > div.container > div.grid-row.content__bg.mgt-16 > div.column-8 > article > div.detail__body.itp_bodycontent_wrapper > div.detail__body-text.itp_bodycontent p",
	"#content > div.container.detail_content.group > div.l_content > div.group > article > div.group.detail_wrap.itp_bodycontent_wrapper > div.itp_bodycontent.detail_text.group p",
	"#detikdetailtext p",
}

func getNews(link string) (original, edited News, err error) {
	// Retry logic
	for retry := 0; retry < 3; retry++ {
		res, err := http.Get(link)
		if err != nil {
			return original, edited, err
		}

		if res.StatusCode == http.StatusNotFound {
			log.Printf("News %s not found. Retrying...\n", link)
			time.Sleep(1 * time.Second) // Wait before retrying
			continue
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return original, edited, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
		}

		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			return original, edited, err
		}

		for _, selector := range titleSelectors {
			original.Title = strings.Join(strings.Fields(doc.Find(selector).Text()), " ")
			edited.Title = original.Title
			if original.Title != "" {
				break
			}
		}

		for _, selector := range bodySelectors {
			var sb strings.Builder
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				if i == 3 || i == 4 {
					return
				}
				text := s.Text()
				text = strings.Join(strings.Fields(text), " ")
				sb.WriteString(text)
				sb.WriteString("\n")
			})

			original.Body = sb.String()
			sb.Reset()
			for _, word := range sastrawi.Tokenize(original.Body) {
				if stopwords.Contains(word) {
					continue
				}

				sb.WriteString(stemmer.Stem(word) + " ")
			}
			edited.Body = stemmer.Stem(sb.String())

			if original.Body != "" {
				break
			}
		}

		original.Link = link
		edited.Link = link

		return original, edited, nil
	}

	return original, edited, fmt.Errorf("maximum retries exceeded")
}

type News struct {
	Title string
	Body  string
	Link  string
}
