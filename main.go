package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/djherbis/times"
	"github.com/dsoprea/go-exif/v2"
	heic "github.com/dsoprea/go-heic-exif-extractor"
	jpeg "github.com/dsoprea/go-jpeg-image-structure"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const ExifDateFormat = "2006:01:02 15:04:05"

var skipFiles = []string{
	"exe",
	"bat",
	"ini",
	"dropbox",
	"app",
}

type Args struct {
	InputPath  string
	OutputPath string
}

type FileInfo struct {
	FileName     string
	FileType     string
	OriginalPath string
	NewPath      string
	CreatedAt    time.Time

	// exif data
	HasExif       bool
	CameraMaker   string
	CameraModel   string
	TimeOriginal  time.Time
	TimeDigitized time.Time
	FileDateTime  time.Time
}

func main() {

	var err error
	var args Args

	flag.StringVar(&args.InputPath, "in", "", "path to working dir")
	flag.StringVar(&args.OutputPath, "out", "", "path to working dir")
	flag.Parse()

	if args.InputPath == "" {
		dir, err := os.Executable()
		args.InputPath = path.Dir(dir)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if args.OutputPath == "" {
		args.OutputPath = args.InputPath + "/sorted"
	}

	log.Println("Input path:", args.InputPath)
	log.Println("Output path:", args.OutputPath)

	if _, err := os.Stat(args.InputPath); os.IsNotExist(err) {
		log.Println("input dir does not exist. stopping.")
		log.Fatalln(err)
	}

	if _, err := os.Stat(args.OutputPath); os.IsNotExist(err) {
		log.Println("output dir does not exist. trying to create.")
		err = os.MkdirAll(args.OutputPath, 0750)

		if err != nil {
			log.Println("error creating output dir. exiting")
			log.Fatalln(err)
		}
	}

	log.Println("LET'S GO")

	files, err := ioutil.ReadDir(args.InputPath)
	if err != nil {
		log.Fatalln(err)
	}

	//var exifData []byte
	var info *FileInfo
	for _, file := range files {
		if file.IsDir() || !file.Mode().IsRegular() {
			continue
		}
		info = new(FileInfo)

		info.OriginalPath = args.InputPath
		info.NewPath = args.OutputPath
		info.FileName = file.Name()
		info.FileType = strings.TrimLeft(strings.ToLower(filepath.Ext(file.Name())), ".")

		skip := false
		for _, ext := range skipFiles {
			if ext == info.FileType {
				skip = true
				break
			}
		}

		if skip {
			continue
		}

		if t, err := times.Stat(info.GetOriginalFilePath()); err == nil {
			info.CreatedAt = t.BirthTime()
		} else {
			info.CreatedAt = file.ModTime()
		}

		switch info.FileType {
		case "jpeg", "jpg", "jpe":
			_ = processExif(info)
		case "heic":
			_ = processHeic(info)
		}

		log.Println(info.GetOriginalFilePath(), "->", info.GetOutputFileName())

		err = os.MkdirAll(info.GetOutputDir(), 0750)
		if err != nil {
			log.Println(err)
		}
		err = os.Rename(info.GetOriginalFilePath(), info.GetOutputFileName())
		if err != nil {
			log.Println(err)
		}
	}
}

func processHeic(info *FileInfo) error {
	mc, parseErr := heic.NewHeicExifMediaParser().ParseFile(info.GetOriginalFilePath())

	if mc != nil {
		_, data, err := mc.Exif()
		if err != nil {
			return err
		}

		et, err := exif.GetFlatExifData(data)

		if err != nil {
			return err
		}
		//tags := rootIfd.EntriesByTagId
		//log.Println(tags)

		processExifTags(info, et)

	} else if parseErr == nil {
		// We should never get a `nil` `mc` value back *and* a `nil`
		// `parseErr`.
		return fmt.Errorf("could not parse JPEG even partially")
	} else {
		return parseErr
	}
	return nil
}

func processExif(info *FileInfo) error {

	intfc, parseErr := jpeg.NewJpegMediaParser().ParseFile(info.GetOriginalFilePath())

	var et []exif.ExifTag

	if intfc != nil {
		// If the parse failed, we should always still get all of the segments
		// that we've encountered so far. It should never be empty, and it
		// should be impossible for it to be `nil`. So, if the parse failed but
		// we still found EXIF data, just ignore the failure and proceed. We had
		// still got what we needed.

		sl := intfc.(*jpeg.SegmentList)

		if len(sl.Segments()) > 0 {
			info.HasExif = true
		}

		var err error
		_, _, et, err = sl.DumpExif()

		// There was a parse error and we couldn't find/parse EXIF data. If the
		// extraction had already failed above and we were just trying for a
		// contingency, fail with that error first.
		if err != nil {
			if parseErr != nil {
				return parseErr
			}
			return err
		}
	} else if parseErr == nil {
		// We should never get a `nil` `intfc` value back *and* a `nil`
		// `parseErr`.
		return fmt.Errorf("could not parse JPEG even partially")
	} else {
		return parseErr
	}

	// If we get here, we either parsed the JPEG file well or at least parsed
	// enough to find EXIF data.

	if et == nil {
		// The JPEG image parsed fine (if it didn't and we haven't yet
		// terminated, we already extracted the EXIF tags above).

		sl := intfc.(*jpeg.SegmentList)

		var err error

		_, _, et, err = sl.DumpExif()
		if err != nil {
			if err == exif.ErrNoExif {
				log.Printf("No EXIF.\n")
			}
			return err
		}
	}

	processExifTags(info, et)

	return nil
}

func processExifTags(info *FileInfo, et []exif.ExifTag) {
	if len(et) == 0 {
		fmt.Printf("EXIF data is present but empty.\n")
	} else {
		info.HasExif = true
		for _, tag := range et {
			tagName := strings.TrimSpace(tag.TagName)

			switch tagName {
			case "Make":
				info.CameraMaker = tag.FormattedFirst
			case "Model":
				info.CameraModel = tag.FormattedFirst
			case "DateTimeOriginal":
				info.TimeOriginal, _ = time.Parse(ExifDateFormat, tag.FormattedFirst)
			case "DateTimeDigitized":
				info.TimeDigitized, _ = time.Parse(ExifDateFormat, tag.FormattedFirst)
			case "FileDateTime":
				atoi, _ := strconv.Atoi(tag.FormattedFirst)
				info.FileDateTime = time.Unix(int64(atoi), 0)
			}
		}
	}
}

func (i *FileInfo) CameraName() string {
	result := ""

	if i.CameraMaker != "" {
		result += strings.TrimSpace(i.CameraMaker) + " "
	}

	if i.CameraModel != "" {
		result += strings.TrimSpace(i.CameraModel)
	}

	result = strings.TrimSpace(result)

	return result
}

// getDateDirName return dir structure for file
func (i *FileInfo) getDateDirName() string {
	date := i.getBestDate()
	return date.Format("2006") + "/" + date.Format("2006-01-02")
}

// getBestDate Get best file date - either when the image was taken or when it was created
func (i *FileInfo) getBestDate() time.Time {

	if !i.TimeOriginal.IsZero() {
		return i.TimeOriginal
	}

	if !i.TimeDigitized.IsZero() {
		return i.TimeDigitized
	}

	if !i.TimeOriginal.IsZero() {
		return i.TimeOriginal
	}

	if !i.CreatedAt.IsZero() {
		return i.CreatedAt
	}

	return time.Now()
}

func (i *FileInfo) GetOutputDir() string {
	result := i.NewPath + "/"

	if !i.HasExif {
		result += "misc/" + i.FileType + "/"
	}

	result += i.getDateDirName() + "/"

	if cam := i.CameraName(); cam != "" {
		result += cam + "/"
	}

	return result
}

func (i *FileInfo) GetOutputFileName() string {

	result := path.Clean(i.GetOutputDir() + i.FileName)
	c := 0
	originalFileName := strings.TrimSuffix(i.FileName, filepath.Ext(i.FileName))

	for {
		if _, err := os.Stat(result); os.IsNotExist(err) {
			break
		}

		c++
		result = path.Clean(i.GetOutputDir() + "/" + originalFileName + "-" + strconv.Itoa(c) + filepath.Ext(i.FileName))
	}
	return result
}

func (i *FileInfo) GetOriginalFilePath() string {
	return i.OriginalPath + "/" + i.FileName
}

func (i *FileInfo) String() string {
	jsonData, _ := json.Marshal(i)
	return string(jsonData)
}
