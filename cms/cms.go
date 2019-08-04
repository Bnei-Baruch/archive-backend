package cms

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

// TODO:
// 1. Add support for active-passive directories

func SyncCMS() {
	cms := viper.GetString("cms.url")
	assets := viper.GetString("cms.assets")

	mkdir(assets+"banners/", 0755)
	mkdir(assets+"persons/", 0755)

	fmt.Println("Syncing Banners...")
	syncBanners(cms, assets)
	fmt.Println("Done")

	fmt.Println("Syncing Persons...")
	syncPersons(cms, assets)
	fmt.Println("Done")
}

type item struct {
	Id      int    `json:"id"`
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Meta    []map[string][]string `json:"meta"`
}

func syncPersons(cms string, assets string) {
	var persons []item
	getItem(cms+"get-posts/persons", &persons)

	for _, person := range persons {
		log.Info(person.Slug)
		checkSlug4Language(person.Slug, "persons-.+?-([a-z]{2})-html")
		saveItem(assets+"persons/"+person.Slug, person)
	}
}

func syncBanners(cms string, assets string) {
	imgURL := viper.GetString("cms.image-url")
	regSetImage, _ := regexp.Compile(imgURL)
	imgSrc := viper.GetString("cms.img-src")

	var banners []item
	getItem(cms+"get-posts/banner", &banners)

	for _, banner := range banners {
		log.Info(banner.Slug)
		checkSlug4Language(banner.Slug, ".+-([a-z]{2})")
		x := checkContent4Image(banner.Content, "<img src=\""+imgURL+"([^\"]+)\"")
		image := x[1]

		// convert images' urls
		banner.Content = regSetImage.ReplaceAllString(banner.Content, imgSrc)

		saveItem(assets+"banners/"+banner.Slug, banner)

		// create directories for images
		mkdir(assets+"images/"+path.Dir(image), 0755)
		saveImage(image, imgURL, assets)
	}
}

func mkdir(dirname string, permissions os.FileMode) {
	if err := os.MkdirAll(dirname, permissions); err != nil {
		log.Fatalf("Unable to create directory: %s", dirname)
		utils.Must(err)
	}
}

func checkSlug4Language(slug string, pattern string) {
	regLang, _ := regexp.Compile(pattern)
	x := regLang.FindStringSubmatch(slug)
	if len(x) == 0 {
		err := fmt.Errorf("\t- slug MUST include language, but does not")
		utils.Must(err)
	}
}

func checkContent4Image(content string, pattern string) (x []string) {
	regFindImage, _ := regexp.Compile(pattern)
	x = regFindImage.FindStringSubmatch(content)
	if len(x) == 0 {
		err := fmt.Errorf("\t- content MUST include language, but does not\n")
		utils.Must(err)
	}
	return
}

func saveItem(path string, v interface{}) {
	m, err := json.Marshal(v)
	utils.Must(err)
	err = ioutil.WriteFile(path, m, 0644)
	utils.Must(err)
}

func getItem(url string, v interface{}) {
	var err error
	client := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	utils.Must(err)
	res, err := client.Do(req)
	utils.Must(err)
	body, err := ioutil.ReadAll(res.Body)
	utils.Must(err)
	err = json.Unmarshal(body, v)
	utils.Must(err)
}

func saveImage(img string, imgURL string, assets string) {
	// copy images
	resp, err := http.Get(imgURL + img)
	utils.Must(err)
	out, err := os.Create(assets + "images/" + img)
	utils.Must(err)
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	utils.Must(err)
	err = resp.Body.Close()
	utils.Must(err)
	err = out.Close()
	utils.Must(err)
}
