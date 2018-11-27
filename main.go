package main

import (
	"fmt"
	"math"
	"net/http"
	"os/signal"
	"sync/atomic"
	"time"

	"os"

	"github.com/PuerkitoBio/goquery"
)

var conf = AppConfiguration{}

type PageResult struct {
	Page      string   `json:"page"`
	Status    int      `json:"status"`
	NestLevel int      `json:"NestLevel"`
	IsValid   bool     `json:"isValid"`
	LinksFrom []string `json:"linksFrom"`
}

var closeFlag = false

//var resultMutex = &sync.Mutex{}
var resultStorage ResultStorage //make(map[string]*PageResult)
var workersAreOver = make(chan bool)
var activeWorkers int64

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	conf = GetAppConfig()
	resultStorage = GetResultStorage()

	ctrlCChan := make(chan os.Signal, 1)
	signal.Notify(ctrlCChan, os.Interrupt)

	timeoutTimer := time.NewTimer(time.Duration(conf.SecToTimeout) * time.Second)

	go ParseURL(conf.Siteurl+conf.CheckingPage, 0)

	for {
		select {
		case <-ctrlCChan:
			fmt.Println("Stopped by Ctr+C!")
			resultStorage.LogResult()
			return
		case <-workersAreOver:
			fmt.Println("Stopped by lack of workers!")
			resultStorage.LogResult()
			return
		case <-timeoutTimer.C:
			fmt.Println("Stopped by timeout!")
			resultStorage.LogResult()
			return
		}
	}
}

func CheckIfWorkersAreOver(url string) {
	atomic.AddInt64(&activeWorkers, -1)
	fmt.Printf("Worker for %s is stopped. Active workers: %v\n", url, activeWorkers)
	if activeWorkers < 1 {
		workersAreOver <- true
		return
	}
}

//ToDo: maybe it's worth sending a PageResult into this func instead of string and integer
func ParseURL(url string, level int) {
	if closeFlag {
		return
	}

	atomic.AddInt64(&activeWorkers, 1)
	defer CheckIfWorkersAreOver(url)

	fmt.Printf("Checking %s, nest level %v, active workers %v\n", url, level, activeWorkers)

	var result = resultStorage.GetSpecResult(url)
	result.IsValid = true

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("http.Get caused an error on %s\n", url)
		result.IsValid = false
		return
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	if math.Round(float64(resp.StatusCode/100)) != 2 {
		result.IsValid = false
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}

	var nextLevel int = level + 1
	if nextLevel > conf.NestLevel {
		return
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		ParseLinkTag(i, s, result, nextLevel)
	})
}

func ParseLinkTag(i int, s *goquery.Selection, localResult *PageResult, nextLevel int) {
	if (conf.LimitPageSearch > 0) && (i > conf.LimitPageSearch) {
		return
	}
	link, err := s.Attr("href")
	if err == false {
		resultStorage.AppendLinkFrom(link, localResult.Page)
	} else {

		var linkDecision = GetLinkDecision(link, nextLevel, conf)

		if linkDecision.IsValid {
			urlWasChecked := resultStorage.AppendLinkFrom(link, localResult.Page)
			if !urlWasChecked {
				go ParseURL(linkDecision.Link, linkDecision.NextNestLevel)
			}
		}
	}
}
