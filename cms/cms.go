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
	log.Info("Source URL: ", cms)
	log.Info("Assets directory: ", assets)

	workDir, err := prepareDirectories(assets)
	if err != nil {
		log.Fatal("prepare directories", err)
	}
	log.Info("Work directory: ", workDir)

	log.Info("Syncing Banners...")
	syncBanners(cms, workDir)

	log.Info("Syncing Persons...")
	syncPersons(cms, workDir)

	// log.Info("Syncing Sources...")
	// syncSources(cms, workDir)
	// log.Info("Done")

	log.Info("Switching Directories...")
	if err = switchDirectories(assets, workDir); err != nil {
		log.Fatal("switch directories", err)
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

func syncPersons(cms string, assets string) {
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
		if err = saveItem(filepath.Join(assets, "persons", person.Slug), person); err != nil {
			log.Fatal("save person", err)
		}
	}
}

func syncBanners(cms string, workDir string) {
	var banners []item
	var err error

	if err = getItem("banners", cms+"get-banners/all", &banners); err != nil {
		log.Fatal("get banners", err)
	}

	imageURL := viper.GetString("cms.image-url")
	for _, banner := range banners {
		log.Info(banner.Slug)
		if err = checkSlug4Language(banner.Slug, bannersLanguage); err != nil {
			log.Fatal(err)
		}

		if err = saveItem(filepath.Join(workDir, "banners", banner.Slug), banner); err != nil {
			log.Fatal("save banner", err)
		}

		// create directories for images
		image := banner.Meta["image"]
		if err = mkdir(0755, workDir, "images", path.Dir(image)); err != nil {
			log.Fatal("make images dir", err)
		}
		if err = saveImage(image, imageURL, workDir); err != nil {
			log.Fatal("save image", err)
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
		err = errors.Wrapf(err,"os.Stat: %s", dirname)
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

	return workDir, nil
}

func switchDirectories(assets, workDir string) (err error) {
	var active = filepath.Join(assets, "active")
	_, err = os.Stat(active)
	if os.IsExist(err) {
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
