package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/davidvartanian/quiet_hn/hn"
)

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.HandleFunc("/", handler(numStories, tpl))

	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	cacheDuration := 5 * time.Minute

	c := ItemCache{
		numStories:      numStories,
		duration:        cacheDuration,
		refreshSubtract: 5 * time.Second,
	}
	go c.Refresh()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		stories, err := c.items()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := templateData{
			Stories: stories,
			Time:    time.Now().Sub(start),
		}
		err = tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

type result struct {
	idx  int
	item item
	err  error
}

func getTopStories(numStories int) ([]item, error) {
	var client hn.Client
	ids, err := client.TopItems()
	if err != nil {
		return nil, errors.New("failed to load top stories")
	}

	var stories []item
	at := 0
	for len(stories) < numStories {
		need := numStories - len(stories)
		stories = append(stories, getStories(ids[at:at+need])...)
		at += need
	}

	return stories, nil
}

func getStories(ids []int) []item {
	numStories := len(ids)
	var stories []item
	resultCh := make(chan result)
	for i := 0; i < numStories; i++ {
		go func(id, idx int) {
			client := hn.Client{}
			hnItem, err := client.GetItem(id)
			if err != nil {
				resultCh <- result{
					err: err,
				}
			} else {
				resultCh <- result{
					idx:  idx,
					item: parseHNItem(hnItem),
				}
			}
		}(ids[i], i)
	}

	results := make([]result, numStories)
	for i := 0; i < numStories; i++ {
		results[i] = <-resultCh
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].idx < results[j].idx
	})

	for _, res := range results {
		if res.err != nil {
			continue
		}
		if isStoryLink(res.item) {
			stories = append(stories, res.item)
		}
	}
	return stories
}

func isStoryLink(item item) bool {
	return item.Type == "story" && item.URL != ""
}

func parseHNItem(hnItem hn.Item) item {
	ret := item{Item: hnItem}
	url, err := url.Parse(ret.URL)
	if err == nil {
		ret.Host = strings.TrimPrefix(url.Hostname(), "www.")
	}
	return ret
}

// item is the same as the hn.Item, but adds the Host field
type item struct {
	hn.Item
	Host string
}

type templateData struct {
	Stories []item
	Time    time.Duration
}
