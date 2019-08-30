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

	newActive, err := switchDirectories(assets)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Link was set to %s\n", newActive)

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

/* Create/clear directories. Return a passive one (i.e. symlink does not point to it) */
func prepareDirectories(assets string) (inactive string, err error) {
	active := filepath.Join(assets, "active")
	if _, err := os.Lstat(active); err == nil {
		link, err := os.Readlink(active)
		if err != nil {
			return "", errors.Wrapf(err, "Unable to read link: %s", link)
		}
		lastLetter := link[len(link)-1:]
		newLetter := "a"
		if lastLetter == "a" {
			newLetter = "b"
			if err = os.RemoveAll(filepath.Join(assets, newLetter)); err != nil {
				return "", errors.Wrapf(err, "Unable to remove directory: %s", newLetter)
			}
			if err = mkdir(0755, assets, newLetter, "banners"); err != nil {
				return "", errors.Wrapf(err, "Unable to create directory for banners: %s", newLetter)
			}
			if err = mkdir(0755, assets, newLetter, "persons"); err != nil {
				return "", errors.Wrapf(err, "Unable to create directory for persons: %s", newLetter)
			}
		} else {
			if err = os.RemoveAll(filepath.Join(assets, newLetter)); err != nil {
				return "", errors.Wrapf(err, "Unable to remove directory: %s", newLetter)
			}
			if err = mkdir(0755, assets, newLetter, "banners"); err != nil {
				return "", errors.Wrapf(err, "Unable to create directory for banners: %s", newLetter)
			}
			if err = mkdir(0755, assets, newLetter, "persons"); err != nil {
				return "", errors.Wrapf(err, "Unable to create directory for persons: %s", newLetter)
			}
		}
		inactive = filepath.Join(assets, newLetter)
	} else {
		if err = mkdir(0755, assets, "a", "banners"); err != nil {
			return "", errors.Wrapf(err, "Unable to create directory for banners: a")
		}
		if err = mkdir(0755, assets, "a", "persons"); err != nil {
			return "", errors.Wrapf(err, "Unable to create directory for persons: a")
		}
		if err = mkdir(0755, assets, "b", "banners"); err != nil {
			return "", errors.Wrapf(err, "Unable to create directory for banners: a")
		}
		if err = mkdir(0755, assets, "b", "persons"); err != nil {
			return "", errors.Wrapf(err, "Unable to create directory for persons: a")
		}

		inactive = filepath.Join(assets, "a")
	}
	return inactive, nil
}

func switchDirectories(assets string) (active string, err error) {
	activeLink := filepath.Join(assets, "active")
	_, err = os.Stat(activeLink)
	if os.IsNotExist(err) {
		// create link to "a" and return it
		err = os.Symlink(filepath.Join(assets, "a"), activeLink)
		if err != nil {
			return "", errors.Wrapf(err, "Unable to create link: %s", activeLink)
		}
		return filepath.Join(assets, "a"), nil
	}

	link, err := os.Readlink(activeLink)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to read link: %s", link)
	}
	// determine the last letter
	// and change it
	if err := os.Remove(activeLink); err != nil {
		return "", fmt.Errorf("failed to unlink: %+v", err)
	}
	lastLetter := link[len(link)-1:]
	newLetter := "a"
	if lastLetter == "a" {
		newLetter = "b"
	}
	inactive := filepath.Join(assets, newLetter)
	err = os.Symlink(inactive, activeLink)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to create link: %s", activeLink)
	}
	return inactive, nil
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
