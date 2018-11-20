package giffer

import (
	"io"
	"github.com/OneOfOne/xxhash"
	"fmt"
	"io/ioutil"
	"path/filepath"
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
	FFmpeg string
	Debug bool
	Out io.Writer
}

// Download the video from URL into Dir and return the full path to the 
// downloaded file.
func (dl Downloader) Download(
	URL string,
	start, end float64,
	q Quality,
) (string, error) {
	// Side channel for loading config because of how the package is
	// unfortunately structured.
	config.FFmpeg = dl.FFmpeg
	dl.logf("ffmpeg: %q\n", dl.FFmpeg)	
	input := fmt.Sprintf("%s_%f_%f_%s", URL, start, end, q)
	h, err := hash(input)
	if err != nil {
		return "", errors.Wrap(err, "creating hash")
	}
	dir := filepath.Join(dl.Dir, h)
	// Side channel for loading config because of how the package is
	// unfortunately structured.
	config.OutputPath = dir
	// Return cached file if it exists.
	entries, _ := ioutil.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), fmt.Sprintf("%s_", q.String())) {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "preparing directories")
	}
	tmp, err := dl.download(URL, start, end, q)
	if err != nil {
		return "", err
	}
	real := filepath.Join(dir, fmt.Sprintf("%s_%s", q, h+filepath.Ext(tmp)))
	if err := os.Rename(tmp, real); err != nil {
		return "", errors.Wrap(err, "renaming temporary file")
	}
	return real, nil
}

func (dl Downloader) logf(f string, v ...interface{}) {
	if !dl.Debug || dl.Out == nil {
		return 
	}
	fmt.Fprintf(dl.Out, f, v...)
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

func (q Quality) String() string {
	switch q {
	case Low: 
		return "low"
	case Medium:
		return "medium"
	case High: 
		return "high"
	case Best:
		return "best"
	default: 
		return "unknown"
	}
}

// download a video from the specified url.
func (dl Downloader) download(
	videoURL string,
	start, end float64,
	quality Quality,
) (string, error) {
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
		cfg := config.NewFromGlobal()
		cfg.FFmpeg = dl.FFmpeg
		d := downloader.Downloader{
			Config: cfg,
			Output: dl.Out,
		}
		path, err = d.Download(&item, videoURL, start, end)
		if err != nil {
			return "", err
		}
	}
	return path, nil
}

func hash(input string) (string, error) {
	h := xxhash.New64()
	if _, err := h.WriteString(input); err != nil {
		return "", errors.Wrap(err, "writing to hash object")
	}
	return fmt.Sprintf("%d", h.Sum64()), nil
}