package main

import "testing"

func GetMockConf() AppConfiguration {
	return AppConfiguration{
		Siteurl:                "somesite",
		ExternalLinksDontCheck: false,
		LimitPageSearch:        1,
	}
}

func TestEmptyLink_IsValid_False(t *testing.T) {
	var l = GetLinkDecision("", 0, GetMockConf())
	if l.IsValid != false {
		t.Error("Expected false, got true")
	}
}

func TestSiteLink_IsValid_True(t *testing.T) {
	var mockConf = GetMockConf()
	var l = GetLinkDecision(mockConf.Siteurl+"/somepage", 0, GetMockConf())
	if l.IsValid != true {
		t.Error("Expected true, got false")
	}
}

func TestExternalLink_DontCheck_IsValid_False(t *testing.T) {
	var mockConf = GetMockConf()
	mockConf.ExternalLinksDontCheck = true
	var l = GetLinkDecision("https:\\smth.com", 0, mockConf)
	if l.IsValid != false {
		t.Error("Expected false, got true.")
	}
}

func TestExternalLink_DoCheck_IsValid_False(t *testing.T) {
	var mockConf = GetMockConf()
	mockConf.ExternalLinksDontCheck = false
	mockConf.NestLevel = 5
	var l = GetLinkDecision("https:\\smth.com", 0, mockConf)
	if l.IsValid != true {
		t.Error("Expected true, got false.")
	}
	if l.NextNestLevel != mockConf.NestLevel {
		t.Error("Nest level assertion failure")
	}
}

func TestLinkDecisionArrayDriven(t *testing.T) {

	var tests = []struct {
		description   string
		link          string
		nest          int
		getconf       func() AppConfiguration
		isValidResult bool
	}{
		{
			"Empty link",
			"",
			1,
			GetMockConf,
			false,
		},
		{
			"internal link",
			"/somepage",
			1,
			GetMockConf,
			true,
		},
		{
			"gibberish",
			"asdasdasdasda",
			1,
			GetMockConf,
			false,
		},
		{
			"external links no check",
			"https:\\smth.com",
			1,
			GetMockConf,
			true,
		},
		{
			"external links check",
			"https:\\smth.com",
			1,
			func() AppConfiguration {
				var mock = GetMockConf()
				mock.ExternalLinksDontCheck = true
				return mock
			},
			false,
		},
	}

	for _, test := range tests {
		var l = GetLinkDecision(test.link, test.nest, test.getconf())
		if l.IsValid != test.isValidResult {
			t.Errorf("Test: %s; got %v; wanted %v", test.description, l.IsValid, test.isValidResult)
		}
	}

}
