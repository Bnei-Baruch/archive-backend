package cms

import (
	"archive/tar"
	"compress/bzip2"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	_ "github.com/spf13/viper"
)

const CONVERTED_TAR = "https://kabbalahmedia.info/assets/converted.tar.xz"

type FileStruct struct {
	Name    string
	Content []byte
}

var httpClient = &http.Client{
	Timeout: 600 * time.Second,
}

func LoadData() {
	var err error

	//cms := viper.GetString("cms.url")
	//assets := viper.GetString("cms.assets")

	if err = processFile(CONVERTED_TAR); err != nil {
		log.Fatal(err)
	}
}

func processFile(urlFile string) (err error) {
	resp, err := httpClient.Get(urlFile)
	if err != nil {
		log.Errorf("Error downloading <%s>, Error: %+v", urlFile, err)
		return err
	}

	defer resp.Body.Close()
	bzf := bzip2.NewReader(resp.Body)
	tarReader := tar.NewReader(bzf)

	var directory string = ""
	var files = []FileStruct{}

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
					log.Errorf("Error handling <%s>, PANIC: %+v", directory, err)
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
				log.Errorf("Error reading file <%s>, PANIC: %+v", urlFile, err)
				return err
			}

			files = append(files, FileStruct{
				Name:    name,
				Content: data,
			})
		default:
			log.Errorf("%s : %c %s %s\n", "Yikes! Unable to figure out type", header.Typeflag, "in file", name)
		}
	}

	if directory != "" {
		err = handleDir(directory, files)
		if err != nil {
			log.Errorf("Error handling <%s>, PANIC: %+v", directory, err)
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
		log.Errorf("Unable to find index.json in directory %s", directory)
		return
	}

	// for each language add new Source to WP
	var indexJson map[string]map[string]string
	err = json.Unmarshal(index.Content, &indexJson)
	if err != nil {
		log.Errorf("json.Unmarshal error (directory %s): %s", directory, err)
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
			log.Errorf("updateWP error (unut %s-%s): %s", unit, language, err)
			return nil
		}
	}

	return
}

func updateWP(unit, language string, file *FileStruct) (err error) {
	fmt.Println("Add to WP: language: ", language, " unit: ", unit, " file(content): ", file.Name, " md5: ", getMD5Hash(file.Content))
	// If this doc already present in WP then compare md5. If different -- update
	// If this doc doesn't present yet -- add it
	return
}

func getMD5Hash(text []byte) string {
	hasher := md5.New()
	hasher.Write(text)
	return hex.EncodeToString(hasher.Sum(nil))
}
