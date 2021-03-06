package backup

//	return os.MkdirAll(p, 0755)

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ushu/udemy-backup/client"
	"github.com/ushu/udemy-backup/client/lister"
)

type Backuper struct {
	Client        *client.Client
	RootDir       string
	LoadSubtitles bool
}

type Asset struct {
	LocalPath string
	RemoteURL string
	Contents  []byte
}

type link struct {
	Title string
	URL   string
}

func New(client *client.Client, rootDir string, loadSubtitles bool) *Backuper {
	return &Backuper{client, rootDir, loadSubtitles}
}

func (b *Backuper) ListCourseAssets(ctx context.Context, course *client.Course) ([]Asset, []string, error) {
	var directories []string
	var assets []Asset

	// then we list all the lectures for the course
	lst := lister.New(b.Client)
	lectures, err := lst.LoadFullCurriculum(ctx, course.ID)
	if err != nil {
		return assets, directories, err
	}
	// we start by creating the necessary directories to hold all the lectures the root dir
	courseDir := getCourseDirectory(b.RootDir, course)
	directories = append(directories, courseDir)

	// now we parse the curriculum
	for _, l := range lectures {
		if chap, ok := l.(*client.Chapter); ok {
			chapDir := getChapterDirectory(b.RootDir, course, chap)
			directories = append(directories, chapDir)
		} else if lecture, ok := l.(*client.Lecture); ok {
			courseAssets, courseDirs := b.ListLectureAssets(course, lecture)
			if err != nil {
				return assets, directories, err
			}
			assets = append(assets, courseAssets...)
			for _, courseDir := range courseDirs {
				directories = append(directories, courseDir)
			}
		}
	}

	return assets, directories, nil
}

func (b *Backuper) ListLectureAssets(course *client.Course, lecture *client.Lecture) ([]Asset, []string) {
	var directories []string
	var assets []Asset

	chapDir := getChapterDirectory(b.RootDir, course, lecture.Chapter)
	prefix := getLecturePrefix(lecture)

	// flag for building the (optional) assets dir
	assetsDirectoryBuilt := false

	// now we traverse the Lecture struct, and enqueue all the necessary work
	// first the video stream, if any
	videos := findVideos(lecture)
	video := filterVideos(videos, 1080)
	if video != nil {
		// enqueue download of the video
		ext := ".mp4"
		if exts, _ := mime.ExtensionsByType(video.Type); len(exts) > 0 {
			ext = exts[0]
		}
		assets = append(assets, Asset{
			LocalPath: filepath.Join(chapDir, prefix+ext),
			RemoteURL: video.File,
		})

		// when the stream is found, we also look up the captions
		if b.LoadSubtitles && lecture.Asset != nil && len(lecture.Asset.Captions) > 0 {
			assetsDir := filepath.Join(chapDir, prefix)
			if !assetsDirectoryBuilt {
				directories = append(directories, assetsDir)
				assetsDirectoryBuilt = true
			}
			for _, c := range lecture.Asset.Captions {
				ext := filepath.Ext(c.FileName)
				locale := c.Locale.Locale
				captionFileName := fmt.Sprintf("%s.%s%s", prefix, locale, ext)
				assets = append(assets, Asset{
					LocalPath: filepath.Join(assetsDir, captionFileName),
					RemoteURL: c.URL,
				})
			}
		}
	}

	// and the audio files
	audio := filterAudio(videos)
	if audio != nil {
		// enqueue download of the audio
		ext := ".mp3"
		if exts, _ := mime.ExtensionsByType(audio.Type); len(exts) > 0 {
			ext = exts[0]
		}
		assets = append(assets, Asset{
			LocalPath: filepath.Join(chapDir, prefix+ext),
			RemoteURL: audio.File,
		})
	}

	// other assets
	otherAssets := findOtherAssets(lecture)
	if len(otherAssets) > 0 {
		assetsDir := filepath.Join(chapDir, prefix)
		if !assetsDirectoryBuilt {
			directories = append(directories, assetsDir)
			assetsDirectoryBuilt = true
		}
		for _, a := range otherAssets {
			assets = append(assets, Asset{
				LocalPath: filepath.Join(assetsDir, lecture.Asset.Title),
				RemoteURL: a.File,
			})
		}
	}

	//
	// additional files
	//
	if len(lecture.SupplementaryAssets) > 0 {
		assetsDir := filepath.Join(chapDir, prefix)
		if !assetsDirectoryBuilt {
			directories = append(directories, assetsDir)
			assetsDirectoryBuilt = true
		}

		var links []*link
		for _, a := range lecture.SupplementaryAssets {
			// we handle links differently
			if a.AssetType == "ExternalLink" {
				links = append(links, &link{
					Title: a.Title,
					URL:   a.ExternalURL,
				})
				continue
			}
			// and we also assets there is something to download
			if a.DownloadUrls == nil {
				continue
			}
			var files []*client.File
			switch a.AssetType {
			case "File":
				files = a.DownloadUrls.File
			case "E-Book":
				files = a.DownloadUrls.Ebook
			}
			// now we grab the file, into the assets directory
			for _, f := range files {
				assets = append(assets, Asset{
					LocalPath: filepath.Join(assetsDir, a.Title),
					RemoteURL: f.File,
				})
			}
		}
		// finally, if we found one or more links, we create a "links.txt" file
		if len(links) > 0 {
			contents := linksToFileContents(links)
			assets = append(assets, Asset{
				LocalPath: filepath.Join(assetsDir, "links.txt"),
				Contents:  contents,
			})
		}
	}

	return assets, directories
}

func findVideos(lecture *client.Lecture) []*client.Video {
	if lecture.Asset.DownloadUrls != nil && len(lecture.Asset.DownloadUrls.Video) > 0 {
		return lecture.Asset.DownloadUrls.Video
	} else if lecture.Asset.StreamUrls != nil {
		return lecture.Asset.StreamUrls.Video
	}
	return nil
}

func findOtherAssets(lecture *client.Lecture) []*client.File {
	a := lecture.Asset
	if a.DownloadUrls == nil {
		return nil
	}
	switch a.AssetType {
	case "File":
		return a.DownloadUrls.File
	case "E-Book":
		return a.DownloadUrls.Ebook
	}
	return nil
}

func filterVideos(videos []*client.Video, resolution int) *client.Video {
	var video *client.Video
	currentRes := 0
	for _, v := range videos {
		if !strings.HasPrefix(v.Type, "video/") {
			continue
		}
		vres, err := strconv.Atoi(v.Label)
		if err != nil {
			continue
		}
		if resolution > 0 && vres == resolution {
			// perfect match
			return v
		}
		// or find the highest resolution available
		if vres > currentRes {
			video = v
			currentRes = vres
		}
	}
	return video
}

func filterAudio(videos []*client.Video) *client.Video {
	for _, v := range videos {
		if !strings.HasPrefix(v.Type, "audio/") {
			return v
		}
	}
	return nil
}

func linksToFileContents(links []*link) []byte {
	w := new(bytes.Buffer)
	for _, link := range links {
		fmt.Fprintln(w, link.Title)
		fmt.Fprintln(w, link.URL)
		fmt.Fprintln(w)
	}
	return w.Bytes()
}
