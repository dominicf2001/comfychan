package util

import (
	"fmt"
	"html/template"
	"image"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/dominicf2001/comfychan/internal/database"
)

// 10 MB memory limit
const FILE_MEM_LIMIT int64 = 10 << 20
const POST_IMG_FULL_PATH = "web/static/img/posts/full"
const POST_IMG_THUMB_PATH = "web/static/img/posts/thumb"

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

func SumUniquePostIps(posts []database.Post) int {
	uniqueIpHashes := map[string]bool{}
	for _, post := range posts {
		uniqueIpHashes[post.IpHash] = true
	}
	return len(uniqueIpHashes)
}
