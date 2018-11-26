package main

import (
	"fmt"
	"math"
	"net/http"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json"
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
var resultMutex = &sync.Mutex{}
var resultStorage = make(map[string]*PageResult)
var workersAreOver = make(chan bool)
var activeWorkers int64

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	conf = GetAppConfig()

	ctrlCChan := make(chan os.Signal, 1)
	signal.Notify(ctrlCChan, os.Interrupt)

	timeoutTimer := time.NewTimer(time.Duration(conf.SecToTimeout) * time.Second)

	go ParseURL(conf.Siteurl+conf.CheckingPage, 0)

	for {
		select {
		case <-ctrlCChan:
			fmt.Println("Stopped by Ctr+C!")
			LogResult()
			return
		case <-workersAreOver:
			fmt.Println("Stopped by lack of workers!")
			LogResult()
			return
		case <-timeoutTimer.C:
			fmt.Println("Stopped by timeout!")
			LogResult()
			return
		}
	}
}

func LogResult() {
	const RESULT_DIR = "./results"
	closeFlag = true
	resultMutex.Lock()
	resJson, _ := json.Marshal(resultStorage)
	_, err := os.Stat(RESULT_DIR)
	if os.IsNotExist(err) {
		os.Mkdir(RESULT_DIR, 0666)
	}

	f, err := os.Create(RESULT_DIR + "/" + conf.ResultPrefix + "_" + strconv.Itoa(int(time.Now().Unix())) + ".json")
	check(err)
	defer f.Close()

	_, err = f.Write(resJson)
	check(err)

	resultMutex.Unlock()
	if !conf.CloseOnFinish {
		fmt.Scan()
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

	resultMutex.Lock()
	var result = PageResult{Page: url, Status: 0, NestLevel: level, IsValid: true}
	resultStorage[url] = &result
	resultMutex.Unlock()

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

func ParseLinkTag(i int, s *goquery.Selection, localResult PageResult, nextLevel int) {
	if (conf.LimitPageSearch > 0) && (i > conf.LimitPageSearch) {
		return
	}
	link, err := s.Attr("href")
	if err == false {
		resultMutex.Lock()
		_, urlWasChecked := resultStorage[link]
		if urlWasChecked {
			resultStorage[link].LinksFrom = append(resultStorage[link].LinksFrom, localResult.Page)
		} else {
			var errResult = PageResult{Page: link, Status: 0, NestLevel: localResult.NestLevel + 1, IsValid: false}
			resultStorage[link] = &errResult
			resultStorage[link].LinksFrom = append(resultStorage[link].LinksFrom, localResult.Page)
		}
		resultMutex.Unlock()
	} else {

		var linkDecision = GetLinkDecision(link, nextLevel)

		if linkDecision.IsValid {
			resultMutex.Lock()
			_, urlWasChecked := resultStorage[linkDecision.Link]
			if urlWasChecked {
				resultStorage[linkDecision.Link].LinksFrom = append(resultStorage[linkDecision.Link].LinksFrom, localResult.Page)
				resultMutex.Unlock()
			} else {
				resultMutex.Unlock()
				go ParseURL(linkDecision.Link, linkDecision.NextNestLevel)
			}
		}
	}
}
