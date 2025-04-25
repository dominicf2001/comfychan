package util

import (
	"fmt"
	"html/template"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/disintegration/imaging"
)

// 10 MB memory limit
const FILE_MEM_LIMIT int64 = 10 << 20
const MAX_REQUEST_BYTES int64 = FILE_MEM_LIMIT + (1 << 20)

var (
	POST_MEDIA_FULL_PATH  = "web/static/media/posts/full"
	POST_MEDIA_THUMB_PATH = "web/static/media/posts/thumb"
)

const MAX_THREAD_COUNT = 50

const MAX_BODY_LEN = 3000
const MAX_SUBJECT_LEN = 50

var SUPPORTED_IMAGE_MIME_TYPES = []string{"image/jpeg", "image/png", "image/gif"}
var SUPPORTED_VIDEO_MIME_TYPES = []string{"video/webm", "video/mp4", "video/ogg"}

type PostMediaType int64

const (
	PostFileImage PostMediaType = iota
	PostFileVideo
	PostFileUnsupported
)

func EnrichPost(body string) string {
	var b strings.Builder
	for _, rawLine := range strings.Split(body, "\n") {
		var outLine string

		for i, rawWord := range strings.Split(rawLine, " ") {
			var outWord string
			if strings.HasPrefix(rawWord, ">>") {
				postId := strings.TrimPrefix(rawWord, ">>")
				esc := template.HTMLEscapeString(rawWord)
				outWord = fmt.Sprintf(
					`<a onclick="onReplyLinkClick(event)" 
						 onmouseover="highlightPost(%[1]s, event)" `+
						`onmouseleave="highlightPost(%[1]s, event, false)" `+
						`href="#post-%[1]s" class="reply-link">%s</a>`,
					postId, esc,
				)
			} else {
				outWord = template.HTMLEscapeString(rawWord)
			}

			if i != 0 {
				outLine += " "
			}
			outLine += outWord
		}

		if strings.HasPrefix(rawLine, ">") && !strings.HasPrefix(rawLine, ">>") {
			outLine = `<span class="greentext">` + outLine + `</span>`
		}

		b.WriteString(outLine)
		b.WriteString("<br/>")
	}
	return b.String()
}

func DetectPostFileType(file multipart.File) (PostMediaType, error) {
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return PostFileUnsupported, err
	}

	fileType := http.DetectContentType(buffer[:n])

	switch {
	case slices.Contains(SUPPORTED_IMAGE_MIME_TYPES, fileType):
		return PostFileImage, nil
	case slices.Contains(SUPPORTED_VIDEO_MIME_TYPES, fileType):
		return PostFileVideo, nil
	default:
		return PostFileUnsupported, nil
	}
}

func SavePostFile(file multipart.File, fileName string) (error, string, string) {
	mediaType, err := DetectPostFileType(file)
	if err != nil {
		return err, "", ""
	}
	file.Seek(0, io.SeekStart)

	fileExt := strings.ToLower(filepath.Ext(fileName))

	// FULL
	if err := os.MkdirAll(POST_MEDIA_FULL_PATH, 0755); err != nil {
		log.Printf("MkdirAll (full): %v", err)
		return err, "", ""
	}

	dstPathFull := filepath.Join(POST_MEDIA_FULL_PATH, fileName)
	dstFull, err := os.Create(dstPathFull)
	if err != nil {
		log.Printf("os.Create (full): %v", err)
		return err, "", ""
	}
	defer dstFull.Close()
	if _, err := io.Copy(dstFull, file); err != nil {
		log.Printf("io.Copy (full): %v", err)
		return err, "", ""
	}

	if _, err := (file).Seek(0, 0); err != nil { // rewind file
		log.Printf("seek file (thumb): %v", err)
		return err, "", ""
	}

	// THUMBNAIL
	dstPathThumb := filepath.Join(POST_MEDIA_THUMB_PATH, fileName)
	if err := os.MkdirAll(POST_MEDIA_THUMB_PATH, 0755); err != nil {
		log.Printf("MkdirAll (thumb): %v", err)
		return err, "", ""
	}

	var thumbFileName string
	if mediaType == PostFileVideo {
		fileNameNoExt := strings.TrimSuffix(fileName, fileExt)
		thumbFileName = fileNameNoExt + ".jpg"

		inputPath := filepath.Join(POST_MEDIA_FULL_PATH, fileName)
		outputPath := filepath.Join(POST_MEDIA_THUMB_PATH, thumbFileName)

		cmd := exec.Command(
			"ffmpeg",
			"-i", inputPath,
			"-ss", "00:00:01.000",
			"-vframes", "1",
			"-vf", "scale=300:-1",
			outputPath,
		)

		if err := cmd.Run(); err != nil {
			log.Printf("ffmpeg error: %v", err)
			return err, "", ""
		}

	} else {
		thumbFileName = fileName

		img, _, err := image.Decode(file)
		if err != nil {
			log.Printf("image.Decode: %v", err)
			return err, "", ""
		}

		var thumb image.Image
		if img.Bounds().Dx() > 300 {
			thumb = imaging.Resize(img, 300, 0, imaging.Lanczos)
		} else {
			thumb = img
		}

		if err = imaging.Save(thumb, dstPathThumb); err != nil {
			log.Printf("imaging.Save: %v", err)
			return err, "", ""
		}
	}

	return nil, fileName, thumbFileName
}

type PostFileInfo struct {
	Size    int64
	Height  int
	Width   int
	IsVideo bool
}

func GetPostFileInfo(mediaPath string) PostFileInfo {
	var result PostFileInfo

	filePath := path.Join(POST_MEDIA_FULL_PATH, mediaPath)

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Failed to get file info at %s: %v", mediaPath, err)
		return PostFileInfo{}
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open file at %s: %v", mediaPath, err)
		return PostFileInfo{}
	}
	defer file.Close()

	// read file type
	mediaType, _ := DetectPostFileType(file)
	file.Seek(0, io.SeekStart)

	result.Size = fileInfo.Size()

	if mediaType != PostFileVideo {
		cfg, _, err := image.DecodeConfig(file)
		if err != nil {
			log.Printf("Failed to decode image at %s: %v", mediaPath, err)
			return PostFileInfo{}
		}

		result.Width = cfg.Width
		result.Height = cfg.Height
		result.IsVideo = false

	} else {
		result.IsVideo = true
	}

	return result
}

func FormatPostFileInfo(fileInfo PostFileInfo) string {
	humanSize := FormatBytes(fileInfo.Size)
	if fileInfo.IsVideo {
		return fmt.Sprintf("(%s)", humanSize)
	}
	return fmt.Sprintf("(%s, %dx%d)", humanSize, fileInfo.Width, fileInfo.Height)
}
