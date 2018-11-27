package main

import "strings"

type LinkDecision struct {
	IsValid       bool
	Link          string
	NextNestLevel int
}

func GetLinkDecision(rawLink string, rawNextLevel int, conf AppConfiguration) LinkDecision {
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
		if conf.ExternalLinksDontCheck {
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
