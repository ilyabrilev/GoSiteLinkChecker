package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json"
	"os"

	"github.com/PuerkitoBio/goquery"
)

var (
	SITEURL               string = "http://lenta.ru"
	CHECKING_PAGE         string = "/"
	MAX_NEST_LEVEL        int    = 3
	SECONDS_TO_TIMEOUT    int    = 30
	CHECK_WORKERS_OFFSET  int    = 5
	KEEP_WORKING          bool   = false
	IGNORE_EXTERNAL_LINKS bool   = false
)

//разбор аргументов командной строки
func init() {
	flag.StringVar(&SITEURL, "s", SITEURL, "URL сайта")
	flag.StringVar(&SITEURL, "p", SITEURL, "Страница для просмотра")
	flag.IntVar(&MAX_NEST_LEVEL, "mn", MAX_NEST_LEVEL, "Максимальная глубина поиска")
	flag.IntVar(&SECONDS_TO_TIMEOUT, "to", SECONDS_TO_TIMEOUT, "Секунд до принудительного завершения")
	flag.IntVar(&CHECK_WORKERS_OFFSET, "wo", CHECK_WORKERS_OFFSET, "Ожидание до начала проверки на отсутствие рабочих воркеров")
	flag.BoolVar(&KEEP_WORKING, "k", KEEP_WORKING, "Производить ли Sleep до ответа от пользователя")
}

type PageResult struct {
	Page      string   `json:"page"`
	Status    int      `json:"status"`
	NestLevel int      `json:"nestLevel"`
	IsValid   bool     `json:"isValid"`
	LinksFrom []string `json:"linksFrom"`
}

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
	flag.Parse()

	key_chan := make(chan os.Signal, 1)
	signal.Notify(key_chan, os.Interrupt)

	//notifChan := make(chan string)

	timeoutTimer := time.NewTimer(time.Duration(SECONDS_TO_TIMEOUT) * time.Second)

	go ParseURL(SITEURL+CHECKING_PAGE, 0)
	//RunWorkersCheck(workersAreOver)

	for {
		select {
		/*
			case notif := <-notifChan:
				fmt.Println(notif)
		*/
		case <-key_chan:
			fmt.Println("Stoped by Ctr+C!")
			LogResult()
			return
		case <-workersAreOver:
			fmt.Println("Stoped by lack of workers!")
			LogResult()
			return
		case <-timeoutTimer.C:
			fmt.Println("Stoped by timeout!")
			LogResult()
			return
		}
	}
}

func LogResult() {
	resultMutex.Lock()
	resJson, _ := json.Marshal(resultStorage)
	f, err := os.Create("./dat1.txt")
	check(err)
	defer f.Close()
	_, err = f.Write(resJson)
	check(err)
	fmt.Printf("%+v\n", resultStorage)
	resultMutex.Unlock()
	if KEEP_WORKING {
		fmt.Scan()
	}
}

func RunWorkersCheck(exitChan chan bool) {
	time.Sleep(time.Duration(CHECK_WORKERS_OFFSET) * time.Second)

	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for range ticker.C {
			if activeWorkers < 1 {
				fmt.Printf("Active workers: %v. Exiting\n", activeWorkers)
				exitChan <- true
				return
			}
			fmt.Printf("Active workers: %v. Continue\n", activeWorkers)
		}
	}()
}

func CheckIfWorkersAreOver(url string) {
	atomic.AddInt64(&activeWorkers, -1)
	fmt.Printf("Worker for %s is stopped. Active workers: %v\n", url, activeWorkers)
	if activeWorkers < 1 {
		workersAreOver <- true
		return
	}
}

func ParseURL(url string, level int) {
	atomic.AddInt64(&activeWorkers, 1)
	defer CheckIfWorkersAreOver(url)

	fmt.Printf("Checking %s, nest level %v, active workers %v\n", url, level, activeWorkers)

	resultMutex.Lock()
	var result = PageResult{Page: url, Status: 0, NestLevel: level, IsValid: true}
	resultStorage[url] = &result
	resultMutex.Unlock()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("http.Get caused an error on %s", url)
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
	if nextLevel > MAX_NEST_LEVEL {
		return
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if i > 30 {
			return
		}
		link, err := s.Attr("href")
		if err == false {
			//нужно ли оповещать о пустых ссылках?
			//fmt.Println(link, err)
			resultMutex.Lock()
			_, urlWasChecked := resultStorage[link]
			if urlWasChecked {
				resultStorage[link].LinksFrom = append(resultStorage[link].LinksFrom, url)
			} else {
				var errResult = PageResult{Page: link, Status: 0, NestLevel: level, IsValid: false}
				resultStorage[link] = &errResult
				resultStorage[link].LinksFrom = append(resultStorage[link].LinksFrom, url)
			}
			resultMutex.Unlock()
		} else {
			parsedLink, valid := GetLink(link)

			var realNextLevel = nextLevel
			if !valid && !IGNORE_EXTERNAL_LINKS {
				realNextLevel = MAX_NEST_LEVEL
			}

			if valid || !IGNORE_EXTERNAL_LINKS {
				resultMutex.Lock()
				_, urlWasChecked := resultStorage[parsedLink]
				if urlWasChecked {
					resultStorage[parsedLink].LinksFrom = append(resultStorage[parsedLink].LinksFrom, url)
					resultMutex.Unlock()
				} else {
					resultMutex.Unlock()
					go ParseURL(parsedLink, realNextLevel)
				}
			}
		}
	})
}

func GetLink(raw string) (string, bool) {
	if strings.HasPrefix(raw, "/") {
		return SITEURL + raw, true
	}
	if strings.HasPrefix(raw, SITEURL) {
		return raw, true
	}
	//fmt.Printf("%s is not valid\n", raw)
	return raw, false
}
