package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adrg/xdg"
	"github.com/mmcdole/gofeed"
	"github.com/pterm/pterm"
)

var logger = slog.New(pterm.NewSlogHandler(&pterm.DefaultLogger))

func main() {
	pterm.DefaultLogger.Level = pterm.LogLevelDebug

	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL("https://subsplease.org/rss/?r=1080")

	rexs, err := getRegexies()
	if err != nil {
		return
	}

	toDownload := filterToDownload(feed.Items, rexs)

	for _, item := range toDownload {
		logger.Debug(item.Title)
	}
}

func getRegexies() ([]*regexp.Regexp, error) {
	dat, err := os.ReadFile(filepath.Join(xdg.ConfigHome, "anime-rss", "titles"))
	if err != nil {
		logger.Error("Failed to read title filters from file", "error", err)
		return nil, err
	}
	var regexies []*regexp.Regexp
	for _, regex_string := range strings.Split(string(dat), "\n") {
		if strings.TrimSpace(regex_string) == "" {
			continue
		}
		re, err := regexp.Compile(regex_string)
		if err != nil {
			logger.Error("Failed to compile regex", "regex", regex_string)
			return nil, errors.New("Failed to compile regex")
		}
		regexies = append(regexies, re)
	}
	return regexies, nil
}

func filterToDownload(items []*gofeed.Item, regexies []*regexp.Regexp) (toDownload []*gofeed.Item) {
	for _, item := range items {
		for _, re := range regexies {
			if re.MatchString(item.Title) {
				toDownload = append(toDownload, item)
				break
			}
		}
	}
	return
}
