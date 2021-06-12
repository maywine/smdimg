package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	mdFile    = flag.String("m", "", "The md file path")
	isInplace = flag.Bool("i", false, "Inplace the edit file, if specified")
)

const (
	mdSuffix       = ".tmp"
	imgIndexPrefix = "img_"
)

type imgInfo struct {
	id      string
	imgByte []byte
}

func main() {
	flag.Parse()
	if len(*mdFile) == 0 {
		log.Fatalf("the md file path must provide\n")
		os.Exit(1)
	}
	readfh, err := os.Open(*mdFile)
	if err != nil {
		log.Fatalf("open md file: %s failed, error: %v\n", *mdFile, err)
		os.Exit(1)
	}
	defer readfh.Close()

	saveFile := *mdFile + mdSuffix
	writefh, err := os.Create(saveFile)
	if err != nil {
		log.Fatalf("creat temp md file: %s failed, error: %v\n", saveFile, err)
		os.Exit(1)
	}
	defer writefh.Close()
	writer := bufio.NewWriter(writefh)

	var imgInfos []imgInfo
	var index int
	//                           1         2         3       4
	reg := regexp.MustCompile("^(.*)(\\!\\[.*\\])\\((.+)\\)(.*)$")
	input := bufio.NewScanner(readfh)
	for input.Scan() {
		line := input.Text()
		result := reg.FindStringSubmatch(line)
		if len(result) > 0 {
			if strings.Contains(result[3], "base64") ||
				strings.Contains(result[3], ":data:image/") {
				continue
			}

			imgENBytes, err := getImage(result[3])
			if err != nil {
				log.Fatalf("get img failed, error: %v\n", err)
				os.Exit(1)
			}

			indexStr := imgIndexPrefix + strconv.FormatInt(int64(index), 10)
			img := imgInfo{
				id:      indexStr,
				imgByte: imgENBytes}
			imgInfos = append(imgInfos, img)
			index++
			writeString(writer, result[1])
			writeString(writer, result[2])
			writeString(writer, fmt.Sprintf("[%s]", indexStr))
			writeString(writer, result[4])
			writeString(writer, "\n")
		} else {
			writeString(writer, line)
			writeString(writer, "\n")
		}
	}

	writeString(writer, "\n\n\n\n\n")
	for _, img := range imgInfos {
		// [index]:data:image/png;base64,iVBORw0
		headByte := []byte(fmt.Sprintf("[%s]:data:image/png;base64,", img.id))
		writeBytes(writer, headByte)
		writeBytes(writer, img.imgByte)
		writeString(writer, "\n")
	}

	err = writer.Flush()
	if err != nil {
		log.Fatalf("failed to flush buffer to file, error: %v\n", err)
		os.Exit(1)
	}

	if *isInplace {
		if err := os.Rename(saveFile, *mdFile); err != nil {
			log.Fatalf("failed to move %s to %s place, error: %v\n", saveFile, *mdFile, err)
			os.Exit(1)
		}
	}
}

func writeBytes(w *bufio.Writer, b []byte) {
	n, err := w.Write(b)
	if err != nil || n != len(b) {
		log.Fatalf("write faile, error: %v\n", err)
		os.Exit(1)
	}
}

func writeString(w *bufio.Writer, str string) {
	n, err := w.WriteString(str)
	if err != nil || n != len(str) {
		log.Fatalf("write faile, error: %v\n", err)
		os.Exit(1)
	}
}

func getImage(imgPath string) ([]byte, error) {
	var reader io.ReadCloser
	f, err := os.Open(imgPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// not local image, try to get image from url
	if f == nil {
		imgPath = strings.TrimPrefix(imgPath, "//")
		if !strings.HasPrefix(imgPath, "http") {
			imgPath = "http://" + imgPath
		}
		resp, err := http.Get(imgPath)
		if err != nil {
			return nil, err
		}
		reader = resp.Body
	} else {
		reader = f
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	var rw io.ReadWriter = new(bytes.Buffer)
	err = png.Encode(rw, img)
	if err != nil {
		return nil, err
	}

	imgw := new(bytes.Buffer)
	encoder := base64.NewEncoder(base64.StdEncoding, imgw)
	defer encoder.Close()

	_, err = io.Copy(encoder, rw)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	err = encoder.Close()
	if err != nil {
		return nil, err
	}
	return imgw.Bytes(), nil
}
