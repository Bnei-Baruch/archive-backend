package cms

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper"
)

type FileStruct struct {
	Name    string
	Content []byte
}

var httpClient = &http.Client{
	Timeout: 600 * time.Second,
}

type wpSource struct {
	Id       int    `json:"id"`
	Slug     string `json:"slug"`
	XSlug    string `json:"xslug"`
	Title    string `json:"title"`
	Uid      string `json:"uid"`
	Language string `json:"language"`
	Md5      string `json:"md5"`
	Content  string `json:"content"`
}

var wpSources = make(map[string]wpSource)

var getPostUrl string
var username string
var password string
var ConvertedTar string

func LoadData() {
	ConvertedTar = viper.GetString("cms.converted-tar")
	getPostUrl = viper.GetString("cms.get-post-url")
	username = viper.GetString("cms.get-post-user")
	password = viper.GetString("cms.get-post-pass")

	if err := loadSources(); err != nil {
		log.Fatal(err)
	}

	if err := processTar(ConvertedTar); err != nil {
		log.Fatal(err)
	}
}

func processTar(urlFile string) error {
	resp, err := httpClient.Get(urlFile)
	if err != nil {
		return errors.Wrapf(err, "Error downloading <%s>, Error: %+v", urlFile, err)
	}

	defer func() {
		x := resp.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "processTar: Close body error %+v", err)
			log.Fatal(err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "wpSave: ReadAll error %+v", err)
	}
	bzf := bzip2.NewReader(bytes.NewReader(body))
	tarReader := tar.NewReader(bzf)

	var directory = ""
	var files = make(map[string]FileStruct)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "Error un-zipping/un-tarring <%s>, Error: %+v", urlFile, err)
		}

		name := header.Name
		switch header.Typeflag {
		case tar.TypeDir:
			if directory != "" {
				if err = handleDir(directory, files); err != nil {
					fmt.Println(errors.Wrapf(err, "Error handling <%s>, Error: %+v", directory, err))
				}
			}
			directory = name
			files = make(map[string]FileStruct)
			continue
		case tar.TypeReg:
			data := make([]byte, header.Size)
			n, err := tarReader.Read(data)
			if n == 0 && err != nil {
				return errors.Wrapf(err, "Error reading file <%s>, Error: %+v", urlFile, err)
			}

			files[name] = FileStruct{
				Name:    name,
				Content: data,
			}
		default:
			return errors.Wrapf(err, "Ups! Unable to figure out type: %c in file %s\n", header.Typeflag, name)
		}
	}

	if directory != "" {
		if err := handleDir(directory, files); err != nil {
			fmt.Println(errors.Wrapf(err, "Error handling <%s>, Error: %+v", directory, err))
		}
	}

	return nil
}

func handleDir(directory string, files map[string]FileStruct) error {
	// Find index.json
	name := fmt.Sprintf("%sindex.json", directory)
	uid := directory[:len(directory)-1]
	index, ok := files[name]
	if ok == false {
		return errors.Wrapf(nil, "handleDir: Unable to find index.json in directory <%s>", directory)
	}

	// for each language add new Source to WP
	var indexJson map[string]map[string]string
	if err := json.Unmarshal(index.Content, &indexJson); err != nil {
		return errors.Wrapf(err, "handleDir: Unmarshal error (directory <%s>): %+v", directory, err)
	}
	if err := updateWP(uid, "xx", &index); err != nil {
		return errors.Wrapf(err, "handleDir: updateWP error (uid %s-%s): %+v", uid, "xx", err)
	}

	for language, x := range indexJson {
		for contentType := range x {
			fileName := fmt.Sprintf("%s%s", directory, x[contentType])
			file, ok := files[fileName]
			if ok == false {
				continue
			}
			if err := updateWP(uid, language, &file); err != nil {
				return errors.Wrapf(err, "handleDir: updateWP error (uid %s-%s): %+v", uid, language, err)
			}
		}
	}

	return nil
}

func updateWP(uid, language string, file *FileStruct) error {
	slug := uid + "-" + language + "-" + file.Name
	md5val := getMD5Hash(file.Content)
	source, ok := wpSources[slug]
	if ok {
		if md5val != source.Md5 {
			// If this doc already present in WP then compare md5. If different -- update
			fmt.Print("u")
			source.Content = string(file.Content)
			source.Md5 = md5val
		} else {
			fmt.Print("s")
			return nil
		}
	} else {
		fmt.Print("a")
		// If this doc doesn't present yet -- add it
		source = wpSource{
			Slug:     slug,
			XSlug:    slug,
			Title:    file.Name,
			Uid:      uid,
			Language: language,
			Md5:      md5val,
			Content:  string(file.Content),
		}
	}
	return wpSave(&source)
}

func wpSave(source *wpSource) error {
	content, err := json.Marshal(source)
	if err != nil {
		return errors.Wrapf(err, "wpSave: Marshal error %+v", err)
	}
	req, err := http.NewRequest(http.MethodPost, getPostUrl+"set-source", bytes.NewBufferString(string(content)))
	if err != nil {
		return errors.Wrapf(err, "wpSave: NewRequest prepare error %+v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.SetBasicAuth(username, password)

	res, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "wpSave: Do POST error %+v", err)
	}
	defer func() {
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "wpSave: Close body error %+v", err)
			log.Fatal(err)
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "wpSave: ReadAll error %+v", err)
	}
	type resType struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	var response resType

	err = json.Unmarshal(body, &response)
	if err != nil {
		return errors.Wrapf(err, "wpSave: Unmarshal response error %+v", err)
	}
	if response.Code != "success" {
		return errors.Wrapf(err, "wpSave: Response error: %s", response.Message)
	}
	return nil
}

func loadSources() error {
	url := getPostUrl + "get-sources/" + "?skip_content=true&page=%d"
	page := 1
	for {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(url, page), nil)
		if err != nil {
			return errors.Wrapf(err, "loadSources: NewRequest prepare error %+v", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		res, err := client.Do(req)
		if err != nil {
			return errors.Wrapf(err, "loadSources: Do GET error %+v", err)
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "loadSources: ReadAll read body error %+v", err)
		}
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "loadSources: Close body error %+v", err)
			log.Fatal(err)
		}
		var sources []wpSource
		err = json.Unmarshal(body, &sources)
		if err != nil {
			return errors.Wrapf(err, "loadSources: Unmarshal error %+v", err)
		}
		if len(sources) == 0 {
			break
		}
		for _, source := range sources {
			wpSources[source.XSlug] = source
		}
		page++
	}
	return nil
}

func getMD5Hash(text []byte) string {
	hash := md5.New()
	hash.Write(text)
	return hex.EncodeToString(hash.Sum(nil))
}
