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
	"strings"
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
	Title    string `json:"title"`
	Unit     string `json:"unit"`
	Language string `json:"language"`
	Md5      string `json:"md5"`
	Content  string `json:"content"`
}

var getPostUrl string
var username string
var password string
var ConvertedTar string

func LoadData() {
	var err error

	ConvertedTar = viper.GetString("cms.converted-tar")
	getPostUrl = viper.GetString("cms.get-post-url")
	username = viper.GetString("cms.get-post-user")
	password = viper.GetString("cms.get-post-pass")

	if err = processFile(ConvertedTar); err != nil {
		log.Fatal(err)
	}
}

func processFile(urlFile string) (err error) {
	resp, err := httpClient.Get(urlFile)
	if err != nil {
		log.Errorf("Error downloading <%s>, Error: %+v", urlFile, err)
		return err
	}

	defer func() {
		x := resp.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "processFile: Close body error %+v", err)
			log.Fatal(err)
		}
	}()

	bzf := bzip2.NewReader(resp.Body)
	tarReader := tar.NewReader(bzf)

	var directory = ""
	var files []FileStruct

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("Error unzipping/untarring <%s>, Error: %+v", urlFile, err)
			return err
		}

		name := header.Name
		switch header.Typeflag {
		case tar.TypeDir:
			if directory != "" {
				err = handleDir(directory, files)
				if err != nil {
					log.Errorf("Error handling <%s>, Error: %+v", directory, err)
					return err
				}
			}
			directory = name
			files = []FileStruct{}
			continue
		case tar.TypeReg:
			data := make([]byte, header.Size)
			n, err := tarReader.Read(data)
			if n == 0 && err != nil {
				log.Errorf("Error reading file <%s>, Error: %+v", urlFile, err)
				return err
			}

			files = append(files, FileStruct{
				Name:    name,
				Content: data,
			})
		default:
			log.Errorf("Ups! Unable to figure out type: %c in file %s\n", header.Typeflag, name)
		}
	}

	if directory != "" {
		err = handleDir(directory, files)
		if err != nil {
			log.Errorf("Error handling <%s>, Error: %+v", directory, err)
			return err
		}
	}

	return
}

func findFile(name string, files []FileStruct) (result *FileStruct) {
	for idx, file := range files {
		if file.Name == name {
			result = &files[idx]
			break
		}
	}

	return
}

func handleDir(directory string, files []FileStruct) (err error) {
	// Find index.json
	name := fmt.Sprintf("%sindex.json", directory)
	index := findFile(name, files)
	if index == nil {
		log.Errorf("handleDir: Unable to find index.json in directory <%s>", directory)
		return
	}

	// for each language add new Source to WP
	var indexJson map[string]map[string]string
	err = json.Unmarshal(index.Content, &indexJson)
	if err != nil {
		log.Errorf("handleDir: Unmarshal error (directory <%s>): %+v", directory, err)
		return nil
	}

	unit := directory[:len(directory)-1]
	for language, x := range indexJson {
		htmlFileName := fmt.Sprintf("%s%s", directory, x["html"])
		file := findFile(htmlFileName, files)
		if file == nil {
			continue
		}
		err = updateWP(unit, language, file)
		if err != nil {
			log.Errorf("handleDir: updateWP error (unit %s-%s): %+v", unit, language, err)
			return nil
		}
	}

	return
}

func updateWP(unit, language string, file *FileStruct) (err error) {
	slug := strings.ToLower(file.Name)
	slug = strings.ReplaceAll(slug, "/", "-")
	slug = strings.ReplaceAll(slug, " .", "-")
	slug = strings.ReplaceAll(slug, ".", "-")

	md5val := getMD5Hash(file.Content)
	err, source := findSource(unit, slug, language)
	if err != nil {
		return
	}
	if source.Id != 0 {
		if md5val != source.Md5 {
			// If this doc already present in WP then compare md5. If different -- update
			fmt.Println("Update: ", file.Name, " - ", language, " - ", unit, " - ", md5val)
			source.Content = string(file.Content)
			source.Md5 = md5val
		} else {
			fmt.Println("Skip: ", file.Name, " - ", language, " - ", unit, " - ", md5val)
			return
		}
	} else {
		// If this doc doesn't present yet -- add it
		fmt.Println("Add: ", file.Name, " - ", language, " - ", unit, " - ", md5val)
		// create new
		source = wpSource{
			Slug:     slug,
			Title:    slug,
			Unit:     unit,
			Language: language,
			Md5:      md5val,
			Content:  string(file.Content),
		}
	}
	err = wpSave(&source)

	return
}

func wpSave(source *wpSource) (err error) {
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
	return
}

func findSource(unit, slug, language string) (err error, source wpSource) {
	req, err := http.NewRequest(http.MethodGet, getPostUrl+"get-source/"+slug+"?skip_content=true", nil)
	if err != nil {
		return errors.Wrapf(err, "findSource: NewRequest prepare error %+v", err), source
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	res, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "findSource: Do GET error %+v", err), source
	}
	defer func() {
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "findSource: Close body error %+v", err)
			log.Fatal(err)
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "findSource: ReadAll read body error %+v", err), source
	}
	var sources []wpSource
	err = json.Unmarshal(body, &sources)
	if err != nil {
		return errors.Wrapf(err, "findSource: Unmarshal error %+v", err), source
	}
	if len(sources) == 0 {
		return
	}
	source = sources[0]
	if source.Language != language || source.Unit != unit {
		return errors.Wrapf(err, "findSource: result has wrong lang/unit %+v", err), source
	}
	return
}

func getMD5Hash(text []byte) string {
	hash := md5.New()
	hash.Write(text)
	return hex.EncodeToString(hash.Sum(nil))
}
