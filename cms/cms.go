package cms

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var client = http.Client{
	Timeout: time.Second * 5, // Maximum of 5 secs
}

func SyncCMS() {
	var err error

	cms := viper.GetString("cms.url")
	assets := viper.GetString("cms.assets")
	imageURL := viper.GetString("cms.image-url")
	assetsImages := viper.GetString("cms.assets-images")
	log.Info("Source URL: ", cms)
	log.Info("Assets directory: ", assets)
	log.Info("Images URL: ", imageURL)
	log.Info("Assets Images directory: ", assetsImages)

	workDir, err := prepareDirectories(assets)
	if err != nil {
		log.Fatal("prepare directories", err)
	}
	log.Info("Work directory: ", workDir)

	log.Info("Syncing Media Library (images) ...")
	syncImages(cms, workDir, imageURL)

	log.Info("Syncing Banners...")
	syncBanners(cms, workDir, imageURL, assetsImages)

	log.Info("Syncing Persons...")
	syncPersons(cms, workDir, imageURL, assetsImages)

	log.Info("Syncing Sources...")
	syncSources(cms, workDir, imageURL, assetsImages)

	log.Info("Switching Directories...")
	if err = switchDirectories(assets, workDir); err != nil {
		log.Fatal("switch directories ", err)
	}

	log.Info("Done")
}

type item struct {
	Id      int               `json:"id"`
	Slug    string            `json:"slug"`
	Title   string            `json:"title"`
	Content string            `json:"content"`
	Meta    map[string]string `json:"meta"`
}

var personsLanguage = regexp.MustCompile("^.+?-([a-z]{2})$")
var bannersLanguage = regexp.MustCompile("^en|he|ru|tr|it|ua|es|de$")

func syncImages(cms string, workDir string, imageURL string) {
	var images []string
	var err error

	if err = getItem("images", cms+"get-images", &images); err != nil {
		log.Fatal("get images ", err)
	}

	for _, image := range images {
		log.Info(image)
		image = strings.Replace(image, imageURL, "", -1)
		// create directories for images
		if err = mkdir(0755, workDir, "images", path.Dir(image)); err != nil {
			log.Fatal("make images dir", err)
		}
		if err = saveImage(image, imageURL, workDir); err != nil {
			log.Fatal("save image", err)
		}
	}
}

func syncPersons(cms string, workDir string, imageURL string, assetsImages string) {
	var persons []item
	var err error

	if err = getItem("persons", cms+"get-persons/all", &persons); err != nil {
		log.Fatal("get persons", err)
	}

	for _, person := range persons {
		log.Info(person.Slug)
		if err = checkSlug4Language(person.Slug, personsLanguage); err != nil {
			log.Fatal(err)
		}
		content := person.Content
		content = strings.Replace(person.Content, imageURL, assetsImages, -1)

		person.Content = content
		if err = saveItem(filepath.Join(workDir, "persons", person.Slug), person); err != nil {
			log.Fatal("save person", err)
		}
	}
}

func syncSources(cms string, workDir string, imageURL string, assetsImages string) {
	for page := 1; ; page++ {
		log.Info("Sources page ", page)

		type sourceT struct {
			Content string `json:"content"`
			Slug    string `json:"xslug"`
		}
		var sources []sourceT
		var err error

		url := fmt.Sprintf("%sget-sources?page=%d", cms, page)
		if err = getItem("sources", url, &sources); err != nil {
			log.Fatal("get sources", err)
		}

		if len(sources) == 0 {
			break
		}

		for _, source := range sources {
			log.Info(source.Slug)
			if err = checkSlug4Language(source.Slug, personsLanguage); err != nil {
				log.Fatal(err)
			}
			matched, _ := regexp.MatchString(".*html$", source.Slug)
			if matched {
				source.Content = strings.Replace(source.Content, imageURL, assetsImages, -1)
			}
			if err = mkdir(0755, workDir, "sources", path.Dir(source.Slug)); err != nil {
				log.Fatal("make images dir", err)
			}
			if err = saveItem(filepath.Join(workDir, "sources", source.Slug), source.Content); err != nil {
				log.Fatal("save source", err)
			}
		}
	}
}

func syncBanners(cms string, workDir string, imageURL string, assetsImages string) {
	var banners []item
	var err error

	if err = getItem("banners", cms+"get-banners/all", &banners); err != nil {
		log.Fatal("get banners ", err)
	}

	for _, banner := range banners {
		log.Info(banner.Slug)
		if err = checkSlug4Language(banner.Slug, bannersLanguage); err != nil {
			log.Fatal(err)
		}

		banner.Meta["image"] = strings.Replace(banner.Meta["image"], imageURL, assetsImages, -1)

		if err = saveItem(filepath.Join(workDir, "banners", banner.Slug), banner); err != nil {
			log.Fatal("save banner", err)
		}
	}
}

func mkdir(permissions os.FileMode, dirs ...string) (err error) {
	dirname := filepath.Join(dirs...)
	_, err = os.Stat(dirname)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(dirname, permissions); err != nil {
			return errors.Wrapf(err, "os.MkdirAll: %s", dirname)
		}
	} else {
		err = errors.Wrapf(err, "os.Stat: %s", dirname)
	}
	return
}

/* Create passive directory */
func prepareDirectories(assets string) (workDir string, err error) {
	workDir = filepath.Join(assets, fmt.Sprint(time.Now().Unix()))

	if err = mkdir(0755, workDir, "banners"); err != nil {
		return "", errors.Wrapf(err, "mkdir banners: %s/banners", workDir)
	}

	if err = mkdir(0755, workDir, "persons"); err != nil {
		return "", errors.Wrapf(err, "mkdir persons: %s/persons", workDir)
	}

	if err = mkdir(0755, workDir, "sources"); err != nil {
		return "", errors.Wrapf(err, "mkdir sources: %s/sources", workDir)
	}

	return workDir, nil
}

func switchDirectories(assets, workDir string) (err error) {
	var active = filepath.Join(assets, "active")
	_, err = os.Lstat(active)
	if err == nil {
		if err = os.Remove(active); err != nil {
			return errors.Wrapf(err, " os.Remove: %s", active)
		}
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return errors.Wrapf(err, "filepath.Abs: %s", workDir)
	}
	if err = os.Symlink(absWorkDir, active); err != nil {
		return errors.Wrapf(err, "os.Symlink: %s to %s", active, absWorkDir)
	}
	return
}

func checkSlug4Language(slug string, pattern *regexp.Regexp) (err error) {
	x := pattern.FindStringSubmatch(slug)
	if len(x) == 0 {
		return errors.Wrapf(err, "\t- slug MUST include language, but does not")
	}

	return
}

func saveItem(path string, v interface{}) (err error) {
	m, err := json.Marshal(v)
	if err != nil {
		return errors.Wrapf(err, "json.Marshal: %s", path)
	}
	err = ioutil.WriteFile(path, m, 0644)
	if err != nil {
		return errors.Wrapf(err, "ioutil.WriteFile: %s", path)
	}
	return
}

func getItem(name string, url string, v interface{}) (err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrapf(err, "http.NewRequest: %s", name)
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "client.Do: %s", name)
	}
	defer func() {
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "res.Body.Close: %s", name)
			log.Fatal(err)
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "ioutil.ReadAll: %s", name)
	}
	err = json.Unmarshal(body, v)
	if err != nil {
		return errors.Wrapf(err, "json.Unmarshal: %s", name)
	}
	return
}

func saveImage(image string, imageURL string, workDir string) (err error) {
	// copy images
	res, err := http.Get(imageURL + image)
	if err != nil {
		return errors.Wrapf(err, "http.Get %s", image)
	}
	out, err := os.Create(filepath.Join(workDir, "images", image))
	if err != nil {
		return errors.Wrapf(err, "os.Create %s", image)
	}
	// Write the body to file
	if _, err = io.Copy(out, res.Body); err != nil {
		return errors.Wrapf(err, "io.Copy %s", image)
	}
	defer func() {
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "saveImage close body")
			log.Fatal(err)
		}
	}()
	if err = out.Close(); err != nil {
		return errors.Wrapf(err, "out.Close %s", image)
	}
	return
}
