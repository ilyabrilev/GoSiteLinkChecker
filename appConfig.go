package main

import (
	"flag"
	"github.com/tkanos/gonfig"
)

var configFile string

type AppConfiguration struct {
	Siteurl                string `json:"siteurl"`
	CheckingPage           string `json:"checkingPage"`
	NestLevel              int    `json:"nestLevel"`
	SecToTimeout           int    `json:"secToTimeout"`
	SecToFirstCheckWorkers int    `json:"secToFirstCheckWorkers"`
	CloseOnFinish          bool   `json:"closeOnFinish"`
	ExternalLinksDontCheck bool   `json:"externalLinksCheck"`
	LimitPageSearch        int    `json:"limitPageSearch"`
	ResultPrefix           string `json:"resultPrefix"`
}

//разбор аргументов командной строки
func init() {
	flag.StringVar(&configFile, "conf", "", "Имя файла настроек, остальные параметры будут перезаписаны")
	flag.StringVar(&conf.Siteurl, "s", conf.Siteurl, "URL сайта")
	flag.StringVar(&conf.Siteurl, "p", conf.Siteurl, "Страница для просмотра")
	flag.IntVar(&conf.NestLevel, "nl", conf.NestLevel, "Максимальная глубина поиска")
	flag.IntVar(&conf.SecToTimeout, "sto", conf.SecToTimeout, "Секунд до принудительного завершения")
	flag.IntVar(&conf.SecToFirstCheckWorkers, "wo", conf.SecToFirstCheckWorkers, "Ожидание до начала проверки на отсутствие рабочих воркеров")
	flag.BoolVar(&conf.CloseOnFinish, "c", conf.CloseOnFinish, "Автоматическое закрытие окна при завершении")
	flag.BoolVar(&conf.ExternalLinksDontCheck, "i", conf.ExternalLinksDontCheck, "Не проверять внешние ссылки")
	flag.IntVar(&conf.LimitPageSearch, "lp", conf.LimitPageSearch, "Ограничение на проверку n ссылок на страницу (0 - нет ограничения)")
	flag.StringVar(&conf.ResultPrefix, "rp", conf.ResultPrefix, "Префикс для сохранения результата")

}

func GetAppConfig() AppConfiguration {
	var conf = AppConfiguration{
		Siteurl:                "http://lenta.ru",
		CheckingPage:           "/",
		NestLevel:              3,
		SecToTimeout:           30,
		SecToFirstCheckWorkers: 5,
		CloseOnFinish:          true,
		ExternalLinksDontCheck: false,
		LimitPageSearch:        30,
		ResultPrefix:           "default",
	}
	flag.Parse()

	if configFile != "" {
		conf = AppConfiguration{}
		err := gonfig.GetConf("./config/"+configFile+".json", &conf)
		if err != nil {
			panic(err)
		}
	}
	return conf
}
