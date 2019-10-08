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

	passive, err := prepareDirectories(assets)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Source URL: ", cms)
	log.Info("Target directory: ", assets)

	log.Info("Syncing Banners...")
	syncBanners(cms, passive)
	log.Info("Done")

	log.Info("Syncing Persons...")
	syncPersons(cms, passive)
	log.Info("Done")

	// log.Info("Syncing Sources...")
	// syncSources(cms, passive)
	// log.Info("Done")

	log.Info("Switching Directories...")
	if err = switchDirectories(assets, passive); err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	for _, person := range persons {
		log.Info(person.Slug)
		if err = checkSlug4Language(person.Slug, personsLanguage); err != nil {
			log.Fatal(err)
		}
		if err = saveItem("person", filepath.Join(assets, "persons", person.Slug), person); err != nil {
			log.Fatal(err)
		}
	}
}

func syncBanners(cms string, assets string) {
	var banners []item
	var err error

	imageURL := viper.GetString("cms.image-url")

	if err = getItem("banners", cms+"get-banners/all", &banners); err != nil {
		log.Fatal(err)
	}

	for _, banner := range banners {
		log.Info(banner.Slug)
		if err = checkSlug4Language(banner.Slug, bannersLanguage); err != nil {
			log.Fatal(err)
		}

		if err = saveItem("banners", filepath.Join(assets, "banners", banner.Slug), banner); err != nil {
			log.Fatal(err)
		}

		// create directories for images
		image := banner.Meta["image"]
		if err = mkdir(0755, assets, "images", path.Dir(image)); err != nil {
			log.Fatal(err)
		}
		if err = saveImage(image, imageURL, assets); err != nil {
			log.Fatal(err)
		}
	}
}

func mkdir(permissions os.FileMode, dirs ...string) (err error) {
	dirname := filepath.Join(dirs...)
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(dirname, permissions); err != nil {
			return errors.Wrapf(err, "Unable to create directory: %s", dirname)
		}
	} else if info.Mode().IsRegular() {
		return errors.Wrapf(err, "Directory already exists as a file: %s", dirname)
	}
	return
}

/* Create passive directory */
func prepareDirectories(assets string) (inactive string, err error) {
	t := time.Now().Unix()
	inactive = filepath.Join(assets, fmt.Sprint(t))
	_, err = os.Stat(inactive)
	if err = mkdir(0755, assets, inactive, "banners"); err != nil {
		return "", errors.Wrapf(err, "Unable to create directory for banners: %s/banners", inactive)
	}
	if err = mkdir(0755, assets, inactive, "persons"); err != nil {
		return "", errors.Wrapf(err, "Unable to create directory for persons: %s/persons", inactive)
	}
	return inactive, nil
}

func switchDirectories(assets, inactive string) (err error) {
	var active = filepath.Join(assets, "active")
	_, err = os.Stat(active)
	if os.IsExist(err) {
		if err = os.Remove(active); err != nil {
			return errors.Wrapf(err, "Unable to remove link %s", active)
		}
	}
	if err = os.Symlink(inactive, active); err != nil {
		return errors.Wrapf(err, "Unable to create link %s to %s", active, inactive)
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

func saveItem(name string, path string, v interface{}) (err error) {
	m, err := json.Marshal(v)
	if err != nil {
		return errors.Wrapf(err, "saveItem::Marshal %s", name)
	}
	err = ioutil.WriteFile(path, m, 0644)
	if err != nil {
		return errors.Wrapf(err, "saveItem::WriteFile %s", name)
	}
	return
}

func getItem(name string, url string, v interface{}) (err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrapf(err, "getItem::NewRequest prepare %s", name)
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "getItem::Do GET %s", name)
	}
	defer func() {
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "getItem::Close close body %s", name)
			log.Fatal(err)
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "getItem::ReadAll read body %s", name)
	}
	err = json.Unmarshal(body, v)
	if err != nil {
		return errors.Wrapf(err, "getItem::Unmarshal unmarshal %s", name)
	}
	return
}

func saveImage(image string, imageURL string, assets string) (err error) {
	// copy images
	res, err := http.Get(imageURL + image)
	if err != nil {
		return errors.Wrapf(err, "saveImage::Get %s", image)
	}
	out, err := os.Create(filepath.Join(assets, "images", image))
	if err != nil {
		return errors.Wrapf(err, "saveImage::Create %s", image)
	}
	// Write the body to file
	if _, err = io.Copy(out, res.Body); err != nil {
		return errors.Wrapf(err, "saveImage::Copy %s", image)
	}
	defer func() {
		x := res.Body.Close()
		if x != nil {
			err = errors.Wrapf(x, "saveImage close body")
			log.Fatal(err)
		}
	}()
	if err = out.Close(); err != nil {
		return errors.Wrapf(err, "saveImage::Close %s", image)
	}
	return
}
