package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"strings"

	"encoding/json"
	"os"

	"github.com/PuerkitoBio/goquery"
)

var (
	SITEURL        string = "http://lenta.ru"
	CHECKING_PAGE  string = "/"
	MAX_NEST_LEVEL int    = 3
)

//разбор аргументов командной строки
func init() {
	flag.StringVar(&SITEURL, "s", SITEURL, "URL сайта")
	flag.IntVar(&MAX_NEST_LEVEL, "mn", MAX_NEST_LEVEL, "Максимальная глубина")
}

type PageResult struct {
	Page      string   `json:"page"`
	Status    int      `json:"status"`
	NestLevel int      `json:"nestLevel"`
	IsValid   bool     `json:"isValid"`
	LinksFrom []string `json:"linksFrom"`
}

var resultStorage = make(map[string]*PageResult)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	flag.Parse()
	ParseURL(SITEURL+CHECKING_PAGE, 0)

	resJson, _ := json.Marshal(resultStorage)
	f, err := os.Create("./dat1.txt")
	check(err)
	defer f.Close()
	_, err = f.Write(resJson)
	check(err)
	fmt.Printf("%+v\n", resultStorage)
}

func ParseURL(url string, level int) {

	fmt.Printf("Checking %s\n", url)

	var result = PageResult{Page: url, Status: 0, NestLevel: level, IsValid: true}
	resultStorage[url] = &result
	//resultStorage[url] = &result

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("http.Get caused an error on %s", url)
		result.IsValid = false
		return
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	if math.Round(float64(resp.StatusCode/100)) != 2 {
		//fmt.Printf("status code error: %d %s", resp.StatusCode, resp.Status)
		result.IsValid = false
		return
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}

	var nextLevel int = level + 1
	if nextLevel > MAX_NEST_LEVEL {
		return
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if i > 10 {
			return
		}
		link, err := s.Attr("href")
		if err == false {
			//нужно ли оповещать о пустых ссылках?
			fmt.Println(link, err)
		} else {
			parsedLink, valid := GetLink(link)
			if valid {
				_, urlWasChecked := resultStorage[parsedLink]
				if urlWasChecked {
					resultStorage[parsedLink].LinksFrom = append(resultStorage[parsedLink].LinksFrom, url)
				} else {
					ParseURL(parsedLink, nextLevel)
				}
			}
		}
	})
}

func GetLink(raw string) (string, bool) {
	if strings.HasPrefix(raw, "/") {
		return SITEURL + raw, true
	}
	if strings.HasPrefix(raw, "http") && strings.HasPrefix(raw, SITEURL) {
		return raw, true
	}
	fmt.Printf("%s is not valid\n", raw)
	return raw, false
}
