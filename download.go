package giffer

import (
	"strings"
	"os"
	"github.com/jackmordaunt/video-downloader/config"
	"github.com/pkg/errors"
	"net/url"

	"github.com/jackmordaunt/video-downloader/downloader"
	"github.com/jackmordaunt/video-downloader/extractors/bcy"
	"github.com/jackmordaunt/video-downloader/extractors/bilibili"
	"github.com/jackmordaunt/video-downloader/extractors/douyin"
	"github.com/jackmordaunt/video-downloader/extractors/douyu"
	"github.com/jackmordaunt/video-downloader/extractors/facebook"
	"github.com/jackmordaunt/video-downloader/extractors/instagram"
	"github.com/jackmordaunt/video-downloader/extractors/iqiyi"
	"github.com/jackmordaunt/video-downloader/extractors/mgtv"
	"github.com/jackmordaunt/video-downloader/extractors/miaopai"
	"github.com/jackmordaunt/video-downloader/extractors/pixivision"
	"github.com/jackmordaunt/video-downloader/extractors/qq"
	"github.com/jackmordaunt/video-downloader/extractors/tumblr"
	"github.com/jackmordaunt/video-downloader/extractors/twitter"
	"github.com/jackmordaunt/video-downloader/extractors/universal"
	"github.com/jackmordaunt/video-downloader/extractors/vimeo"
	"github.com/jackmordaunt/video-downloader/extractors/weibo"
	"github.com/jackmordaunt/video-downloader/extractors/youku"
	"github.com/jackmordaunt/video-downloader/extractors/youtube"
	"github.com/jackmordaunt/video-downloader/utils"
)

// Downloader is responsible for downloading videos.
type Downloader struct {
	Dir string
}

// Download the video from URL into Dir and return the full path to the 
// downloaded file.
func (dl Downloader) Download(URL string, q Quality) (string, error) {
	if err := os.MkdirAll(dl.Dir, 0755); err != nil && err != os.ErrExist {
		return "", errors.Wrap(err, "preparing directories")
	}
	// Side channel for loading config because of how the package is
	// unfortunately structured.
	config.OutputPath = dl.Dir
	return Download(URL, q)
}

// Quality is an enum representing the various video qualities.
type Quality int 

// Matches returns true if the input represents the quality as a string.
// This is primarily an adaptor so we don't have to change the video-downloader
// package.
func (q Quality) Matches(str string) bool {
	var patterns []string
	switch q {
	case Best: 
		patterns = append(patterns, "1080p")
	case High: 
		patterns = append(patterns, "720p", "480p")
	case Medium: 
		patterns = append(patterns, "360p")
	case Low: 
		patterns = append(patterns, "240p", "144p")
	}
	for _, p := range patterns {
		if strings.Contains(str, p) {
			return true
		}
	}
	return false
}

const (
	// Low 144p
	Low Quality = iota
	// Medium 360p
	Medium 
	// High 720p
	High
	// Best 1080p@60 > 720p@60 > 1080p > 720p
	Best
)

// Download a video from the specified url.
func Download(videoURL string, quality Quality) (string, error) {
	var (
		domain string
		err    error
		data   []downloader.Data
	)
	bilibiliShortLink := utils.MatchOneOf(videoURL, `^(av|ep)\d+`)
	if bilibiliShortLink != nil {
		bilibiliURL := map[string]string{
			"av": "https://www.bilibili.com/video/",
			"ep": "https://www.bilibili.com/bangumi/play/",
		}
		domain = "bilibili"
		videoURL = bilibiliURL[bilibiliShortLink[1]] + videoURL
	} else {
		u, err := url.ParseRequestURI(videoURL)
		if err != nil {
			return "", errors.Wrap(err, "parsing uri")
		}
		domain = utils.Domain(u.Host)
	}
	switch domain {
	case "douyin", "iesdouyin":
		data, err = douyin.Download(videoURL)
	case "bilibili":
		data, err = bilibili.Download(videoURL)
	case "bcy":
		data, err = bcy.Download(videoURL)
	case "pixivision":
		data, err = pixivision.Download(videoURL)
	case "youku":
		data, err = youku.Download(videoURL)
	case "youtube", "youtu": // youtu.be
		data, err = youtube.Download(videoURL)
	case "iqiyi":
		data, err = iqiyi.Download(videoURL)
	case "mgtv":
		data, err = mgtv.Download(videoURL)
	case "tumblr":
		data, err = tumblr.Download(videoURL)
	case "vimeo":
		data, err = vimeo.Download(videoURL)
	case "facebook":
		data, err = facebook.Download(videoURL)
	case "douyu":
		data, err = douyu.Download(videoURL)
	case "miaopai":
		data, err = miaopai.Download(videoURL)
	case "weibo":
		data, err = weibo.Download(videoURL)
	case "instagram":
		data, err = instagram.Download(videoURL)
	case "twitter":
		data, err = twitter.Download(videoURL)
	case "qq":
		data, err = qq.Download(videoURL)
	default:
		data, err = universal.Download(videoURL)
	}
	if err != nil {
		return "", errors.Wrap(err, "preparing")
	}
	var path string
	for _, item := range data {
		if item.Err != nil {
			return "", errors.Wrap(item.Err, "extracting")
		}
		for k, stream := range item.Streams {
			if quality.Matches(stream.Quality) {
				config.Stream = k
				break
			}
		}
		path, err = item.Download(videoURL)
		if err != nil {
			return "", errors.Wrap(err, "downloading")
		}
	}
	return path, nil
}
