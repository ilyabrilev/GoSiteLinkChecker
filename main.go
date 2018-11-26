package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json"
	"os"

	"github.com/PuerkitoBio/goquery"
	"github.com/tkanos/gonfig"
)

type AppConfiguration struct {
	Siteurl                string `json:"siteurl"`
	CheckingPage           string `json:"checkingPage"`
	NestLevel              int    `json:"nestLevel"`
	SecToTimeout           int    `json:"secToTimeout"`
	SecToFirstCheckWorkers int    `json:"secToFirstCheckWorkers"`
	CloseOnFinish          bool   `json:"closeOnFinish"`
	ExternalLinksCheck     bool   `json:"externalLinksCheck"`
	LimitPageSearch        int    `json:"limitPageSearch"`
	ResultPrefix           string `json:"resultPrefix"`
}

var conf = AppConfiguration{
	Siteurl:                "http://lenta.ru",
	CheckingPage:           "/",
	NestLevel:              3,
	SecToTimeout:           30,
	SecToFirstCheckWorkers: 5,
	CloseOnFinish:          true,
	ExternalLinksCheck:     false,
	LimitPageSearch:        30,
	ResultPrefix:           "default",
}

var configFile string

//разбор аргументов командной строки
func init() {
	flag.StringVar(&configFile, "conf", "", "Имя файла настроек, остальные параметры будут перезаписаны")
	flag.StringVar(&conf.Siteurl, "s", conf.Siteurl, "URL сайта")
	flag.StringVar(&conf.Siteurl, "p", conf.Siteurl, "Страница для просмотра")
	flag.IntVar(&conf.NestLevel, "nl", conf.NestLevel, "Максимальная глубина поиска")
	flag.IntVar(&conf.SecToTimeout, "sto", conf.SecToTimeout, "Секунд до принудительного завершения")
	flag.IntVar(&conf.SecToFirstCheckWorkers, "wo", conf.SecToFirstCheckWorkers, "Ожидание до начала проверки на отсутствие рабочих воркеров")
	flag.BoolVar(&conf.CloseOnFinish, "c", conf.CloseOnFinish, "Автоматическое закрытие окна при завершении")
	flag.BoolVar(&conf.ExternalLinksCheck, "i", conf.ExternalLinksCheck, "Не проверять внешние ссылки")
	flag.IntVar(&conf.LimitPageSearch, "lp", conf.LimitPageSearch, "Ограничение на проверку n ссылок на страницу (0 - нет ограничения)")
	flag.StringVar(&conf.ResultPrefix, "rp", conf.ResultPrefix, "Префикс для сохранения результата")

}

type PageResult struct {
	Page      string   `json:"page"`
	Status    int      `json:"status"`
	NestLevel int      `json:"NestLevel"`
	IsValid   bool     `json:"isValid"`
	LinksFrom []string `json:"linksFrom"`
}

type LinkDecision struct {
	IsValid       bool
	Link          string
	NextNestLevel int
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
	flag.Parse()

	if configFile != "" {
		conf = AppConfiguration{}
		err := gonfig.GetConf("./config/"+configFile+".json", &conf)
		if err != nil {
			panic(err)
		}
	}

	key_chan := make(chan os.Signal, 1)
	signal.Notify(key_chan, os.Interrupt)

	timeoutTimer := time.NewTimer(time.Duration(conf.SecToTimeout) * time.Second)

	go ParseURL(conf.Siteurl+conf.CheckingPage, 0)

	for {
		select {
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
	//fmt.Printf("%+v\n", resultStorage)
	resultMutex.Unlock()
	if !conf.CloseOnFinish {
		fmt.Scan()
	}
}

func RunWorkersCheck(exitChan chan bool) {
	time.Sleep(time.Duration(conf.SecToFirstCheckWorkers) * time.Second)

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

func GetLinkDecision(rawLink string, rawNextLevel int) LinkDecision {
	var retLink = LinkDecision{IsValid: true, Link: rawLink, NextNestLevel: rawNextLevel}
	if IsLinkInBlackList(rawLink) {
		retLink.IsValid = false
		return retLink
	}
	//внутренние ссылки, начинающиеся со слеша должны быть дополнены URL сайта
	if strings.HasPrefix(rawLink, "/") {
		retLink.Link = conf.Siteurl + rawLink
		return retLink
	}
	//внутренние ссылки с полным путем
	if strings.HasPrefix(rawLink, conf.Siteurl) {
		return retLink
	}
	//внешние ссылки
	if strings.HasPrefix(rawLink, "http") || strings.HasPrefix(rawLink, "www") {
		retLink.NextNestLevel = conf.NestLevel
		if conf.ExternalLinksCheck {
			retLink.IsValid = false
		}
		return retLink
	}
	//не пойми что
	retLink.IsValid = false
	return retLink
}

func IsLinkInBlackList(link string) bool {
	return false
}

func GetLink(raw string) (string, bool) {
	if strings.HasPrefix(raw, "/") {
		return conf.Siteurl + raw, true
	}
	if strings.HasPrefix(raw, conf.Siteurl) {
		return raw, true
	}
	return raw, false
}
