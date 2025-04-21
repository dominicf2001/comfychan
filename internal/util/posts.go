package util

import (
	"fmt"
	"html/template"
	"image"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

// 10 MB memory limit
const FILE_MEM_LIMIT int64 = 10 << 20
const POST_IMG_FULL_PATH = "web/static/img/posts/full"
const POST_IMG_THUMB_PATH = "web/static/img/posts/thumb"
const MAX_THREAD_COUNT = 50

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

func SavePostFile(file *multipart.File, filename string) error {
	dstPathFull := filepath.Join(POST_IMG_FULL_PATH, filename)
	dstPathThumb := filepath.Join(POST_IMG_THUMB_PATH, filename)

	// FULL
	if err := os.MkdirAll(POST_IMG_FULL_PATH, 0755); err != nil {
		log.Printf("MkdirAll (full): %v", err)
		return err
	}

	dstFull, err := os.Create(dstPathFull)
	if err != nil {
		log.Printf("os.Create (full): %v", err)
		return err
	}
	defer dstFull.Close()
	if _, err := io.Copy(dstFull, *file); err != nil {
		log.Printf("io.Copy (full): %v", err)
		return err
	}

	if _, err := (*file).Seek(0, 0); err != nil { // rewind file
		log.Printf("seek file (thumb): %v", err)
		return err
	}

	// THUMBNAIL
	if err := os.MkdirAll(POST_IMG_THUMB_PATH, 0755); err != nil {
		log.Printf("MkdirAll (thumb): %v", err)
		return err
	}

	img, _, err := image.Decode(*file)
	if err != nil {
		log.Printf("image.Decode: %v", err)
		return err
	}

	thumb := imaging.Resize(img, 300, 0, imaging.Lanczos)
	if err = imaging.Save(thumb, dstPathThumb); err != nil {
		log.Printf("imaging.Save: %v", err)
		return err
	}

	return nil
}

type PostImageInfo struct {
	Size   int64
	Height int
	Width  int
}

func GetPostImageInfo(mediaPath string) PostImageInfo {
	var result PostImageInfo

	imagePath := path.Join(POST_IMG_FULL_PATH, mediaPath)

	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		log.Printf("Failed to get image file info at %s: %v", mediaPath, err)
		return PostImageInfo{}
	}

	file, err := os.Open(imagePath)
	if err != nil {
		log.Printf("Failed to open image at %s: %v", mediaPath, err)
		return PostImageInfo{}
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		log.Printf("Failed to decode image at %s: %v", mediaPath, err)
		return PostImageInfo{}
	}

	result.Size = fileInfo.Size()
	result.Width = cfg.Width
	result.Height = cfg.Height

	return result
}

func FormatBytes(bytes int64) string {
	const (
		KB = 1 << 10 // 1024
		MB = 1 << 20
		GB = 1 << 30
		TB = 1 << 40
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
