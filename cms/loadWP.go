package cms

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
	XContent []byte `json:"xcontent"`
}

var wpSources = make(map[string]wpSource)

type ConfigWP struct {
	getPostUrl   string
	username     string
	password     string
	convertedTar string
}

var configWP ConfigWP

var tmpDir string
var workDir string

func loadConfigWP() {
	configWP.convertedTar = viper.GetString("cms.converted-tar")
	configWP.getPostUrl = viper.GetString("cms.get-post-url")
	configWP.username = viper.GetString("cms.get-post-user")
	configWP.password = viper.GetString("cms.get-post-pass")
}

func LoadData() {
	var err error

	loadConfigWP()

	if err = loadSources(); err != nil {
		log.Fatal(err)
	}

	if err = prepareTempDirTree("loadWP"); err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	if err = os.Chdir(tmpDir); err != nil {
		log.Fatal(err)
	}

	if err = prepareTarFile(); err != nil {
		log.Fatal(err)
	}

	if err = processFiles(); err != nil {
		log.Fatal(err)
	}

}

func prepareTempDirTree(tree string) (err error) {
	tmpDir, err = ioutil.TempDir("", "")
	if err != nil {
		return errors.Wrapf(err, "Unable to calculate temp directory: %v\n", err)
	}

	workDir = filepath.Join(tmpDir, tree)
	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return errors.Wrapf(err, "Unable to create temp directory: %v\n", err)
	}

	return
}

func readFiles(directory string) (files map[string]FileStruct, err error) {
	files = make(map[string]FileStruct)
	list, _ := readDirectory(directory)
	for _, name := range list {
		fname := fmt.Sprintf("%s/%s", directory, name)
		fd, err := os.Open(fname)
		if err != nil {
			return files, errors.Wrapf(err, "Unable to open file <%s>, Error: %+v\n", name, err)
		}
		info, _ := fd.Stat()
		data := make([]byte, info.Size())
		n, err := fd.Read(data)
		if err != nil || int64(n) != info.Size() {
			return files, errors.Wrapf(err, "Unable to read file <%s>, Error: %+v\n", name, err)
		}
		files[name] = FileStruct{
			Name:    name,
			Content: data,
		}
	}

	return
}

func readDirectory(dirName string) (list []string, err error) {
	file, err := os.Open(dirName)
	if err != nil {
		return list, errors.Wrapf(err, "Unable to open <%s>, Error: %+v", dirName, err)
	}
	defer file.Close()

	list, err = file.Readdirnames(0) // 0 to read all folders
	if err != nil {
		return list, errors.Wrapf(err, "Unable to read <%s>, Error: %+v", dirName, err)
	}
	return
}

func processFiles() (err error) {
	directories, err := readDirectory(workDir)
	if err != nil {
		return
	}

	for _, uid := range directories {
		directory := fmt.Sprintf("%s/%s", workDir, uid)
		files, err := readFiles(directory)
		if err != nil {
			return errors.Wrapf(err, "Error reading <%s>, Error: %+v", directory, err)
		}
		if err = handleDir(uid, files); err != nil {
			return errors.Wrapf(err, "Error handling <%s>, Error: %+v", directory, err)
		}
	}

	return
}

func prepareTarFile() (err error) {
	urlFile := configWP.convertedTar
	fmt.Printf("Preparing tar file <%s>...\n", urlFile)
	resp, err := httpClient.Get(urlFile)
	if err != nil {
		return errors.Wrapf(err, "Unable to download <%s>, Error: %+v", urlFile, err)
	}
	defer resp.Body.Close()

	tarName := path.Base(resp.Request.URL.String())

	// Create the file
	out, err := os.Create(tarName)
	if err != nil {
		return errors.Wrapf(err, "Unable to create <%s>, Error: %+v", urlFile, err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return errors.Wrapf(err, "Unable to copy <%s>, Error: %+v", urlFile, err)
	}
	cmd := exec.Command("tar", "xvzf", tarName, "--one-top-level="+workDir)
	err = cmd.Run()

	return errors.Wrapf(err, "Unable to extract files from <%s>, Error: %+v", urlFile, err)
}

func handleDir(uid string, files map[string]FileStruct) error {
	// Find index.json
	index, ok := files["index.json"]
	if ok == false {
		return errors.Wrapf(nil, "handleDir: Unable to find index.json in directory <%s>", uid)
	}

	// for each language add new Source to WP
	var indexJson map[string]map[string]string
	if err := json.Unmarshal(index.Content, &indexJson); err != nil {
		return errors.Wrapf(err, "handleDir: Unmarshal error (directory <%s>): %+v", uid, err)
	}
	if err := updateWP(uid, "en", &index); err != nil {
		return errors.Wrapf(err, "handleDir: updateWP error (uid %s-%s): %+v", uid, "en", err)
	}

	for language, x := range indexJson {
		for contentType := range x {
			file, ok := files[x[contentType]]
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
	slug := fmt.Sprintf("%s-%s-%s", uid, language, file.Name)
	md5val := getMD5Hash(file.Content)
	source, ok := wpSources[slug]
	if ok {
		if md5val != source.Md5 {
			// If this doc already present in WP then compare md5. If different -- update
			fmt.Print("u")
			source.Md5 = md5val
		} else {
			fmt.Print(".")
			return nil
		}
	} else {
		fmt.Print("a")
		// If this doc doesn't present yet -- add it
		source = wpSource{
			Slug:     slug,
			XSlug:    slug,
			Title:    slug,
			Uid:      uid,
			Language: language,
			Md5:      md5val,
		}
	}
	if filepath.Ext(file.Name) == ".html" {
		source.Content = string(file.Content)
	} else {
		source.XContent = file.Content
	}
	return wpSave(&source)
}

func wpSave(source *wpSource) error {
	content, err := json.Marshal(source)
	if err != nil {
		return errors.Wrapf(err, "wpSave: Marshal error %+v", err)
	}
	req, err := http.NewRequest(http.MethodPost, configWP.getPostUrl+"set-source", bytes.NewBufferString(string(content)))
	if err != nil {
		return errors.Wrapf(err, "wpSave: NewRequest prepare error %+v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.SetBasicAuth(configWP.username, configWP.password)

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
	url := fmt.Sprintf("%sget-sources/?skip_content=true&page=%%d", configWP.getPostUrl)
	fmt.Print("Loading sources")
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
		fmt.Print(".")
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
	fmt.Println("")
	return nil
}

func getMD5Hash(text []byte) string {
	hash := md5.New()
	hash.Write(text)
	return hex.EncodeToString(hash.Sum(nil))
}
