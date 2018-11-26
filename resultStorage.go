package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

type ResultStorage struct {
	results     map[string]*PageResult
	resultMutex *sync.Mutex
}

func GetResultStorage() ResultStorage {
	var ret = ResultStorage{}
	ret.results = make(map[string]*PageResult)
	ret.resultMutex = &sync.Mutex{}
	return ret
}

func (r *ResultStorage) Lock() {
	r.resultMutex.Lock()
}

func (r *ResultStorage) Unlock() {
	r.resultMutex.Unlock()
}

func (r *ResultStorage) Add(pr *PageResult, url string) {
	r.results[url] = pr
}

func (r *ResultStorage) AppendLinkFrom(link string, page string) bool {
	r.Lock()
	_, urlWasChecked := r.results[link]
	if urlWasChecked {
		r.results[link].LinksFrom = append(r.results[link].LinksFrom, page)
	} else {
		var errResult = PageResult{Page: link, Status: 0, NestLevel: 0, IsValid: false}
		r.results[link] = &errResult
		r.results[link].LinksFrom = append(r.results[link].LinksFrom, page)
	}
	r.Unlock()
	return urlWasChecked
}

func (r *ResultStorage) GetSpecResult(link string) *PageResult {
	r.Lock()
	result, isResultExist := r.results[link]
	if !isResultExist {
		//ToDo: decide return or panic?
		result = &PageResult{Page: link, Status: 0, NestLevel: 0, IsValid: false}
	}
	r.Unlock()
	return result
}

func (r *ResultStorage) LogResult() {
	const RESULT_DIR = "./results"
	closeFlag = true
	r.Lock()
	resJson, _ := json.Marshal(r.results)
	_, err := os.Stat(RESULT_DIR)
	if os.IsNotExist(err) {
		os.Mkdir(RESULT_DIR, 0666)
	}

	f, err := os.Create(RESULT_DIR + "/" + conf.ResultPrefix + "_" + strconv.Itoa(int(time.Now().Unix())) + ".json")
	check(err)
	defer f.Close()

	_, err = f.Write(resJson)
	check(err)

	r.Unlock()
	if !conf.CloseOnFinish {
		fmt.Scan()
	}
}
