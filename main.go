package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/mmcdole/gofeed"
	"github.com/pterm/pterm"
	"github.com/siku2/arigo"
)

var logger = slog.New(pterm.NewSlogHandler(&pterm.DefaultLogger))

type DownloadItem struct {
	feedItem    *gofeed.Item
	gid         arigo.GID
	progressBar *pterm.ProgressbarPrinter
}

func main() {
	pterm.DefaultLogger.Level = pterm.LogLevelDebug

	aria2, err := arigo.Dial("ws://localhost:6800/jsonrpc", "")
	if err != nil {
		logger.Error("Failed to connect to Aria2 RPC client", "error", err)
		return
	}

	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL("https://subsplease.org/rss/?r=1080")

	rexs, err := getRegexies()
	if err != nil {
		return
	}

	toDownload := filterToDownload(feed.Items, rexs)

	// A magnet download is handled in 2 steps by aria2. First, it downloads
	// the metadata from the magnet link. Once it is completed, secondly, it
	// downloads the actual file using that metadata.
	//
	// The first download will be done directly with the GID that is returned
	// by `AddURI`, however this will not track the download of the actual
	// file; i.e. this download ONLY manages the metadata from the magnet link.
	// The second download, will have a "following" field set to the GID of the
	// first download.
	//
	// Therefore, to properly track the download, we need to track the GID and
	// ALSO the "following" of an active download so that we know in which of
	// the 2 stages of the download we are in.
	for idx := range toDownload {
		item := &toDownload[idx]
		gid, err := aria2.AddURI(arigo.URIs(item.feedItem.Link), nil)
		if err != nil {
			logger.Error("Failed to add download", "title", item.feedItem.Title, "error", err)
			continue
		}
		item.gid = gid
	}

	multi := prepareProgressBars(toPointerArray(toDownload))

	multi.Start()
	defer multi.Stop()

	logger.Info("Waiting for downloads to have started...")
	// Wait a little for downloads to actually start
	time.Sleep(time.Second * 5)

	for {
		activeDownloads, err := aria2.TellActive("following", "gid", "completedLength", "totalLength")
		if err != nil {
			logger.Error("Failed to get active downloads from aria2c", "error", err)
		}
		// Only the downloads that we submitted to aria2 ourselves
		var relevantDownloads []*arigo.Status
		for _, item := range toDownload {
			for _, status := range activeDownloads {
				var progress int = 0
				if status.TotalLength > 0 {
					progress = int(100 * (float32(status.CompletedLength) / float32(status.TotalLength)))
				}
				if status.GID == item.gid.GID || status.Following == item.gid.GID {
					relevantDownloads = append(relevantDownloads, &status)
					if item.progressBar != nil {
						item.progressBar.Current = int(progress)
					}
				}
			}
		}
		if len(relevantDownloads) == 0 {
			logger.Info("Finished downloading")
			break
		}
		time.Sleep(time.Millisecond * 25)
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

func filterToDownload(items []*gofeed.Item, regexies []*regexp.Regexp) (toDownload []DownloadItem) {
	for _, item := range items {
		for _, re := range regexies {
			if re.MatchString(item.Title) {
				toDownload = append(toDownload, DownloadItem{feedItem: item})
				break
			}
		}
	}
	return
}

func prepareProgressBars(toDownload []*DownloadItem) pterm.MultiPrinter {
	multi := pterm.DefaultMultiPrinter
	for idx, item := range toDownload {
		pbar, err := pterm.
			DefaultProgressbar.
			WithTotal(100).
			WithWriter(multi.NewWriter()).
			WithMaxWidth(200).
			Start(item.feedItem.Title)
		if err != nil {
			logger.Error("Failed to start progress bar", "i", idx, "error", err)
			continue
		}
		item.progressBar = pbar
	}
	return multi
}

func toPointerArray[T any](slice []T) []*T {
	pointers := make([]*T, len(slice))
	for i := range slice {
		pointers[i] = &slice[i]
	}
	return pointers
}
