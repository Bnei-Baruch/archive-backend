package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"
)

// TODO:
// 1. Add support for active-passive directories

var cmsCmd = &cobra.Command{
	Use:   "cms",
	Short: "Sync data from CMS",
	Run: func(cmd *cobra.Command, args []string) {
		cms := viper.GetString("cms.url")
		assets := viper.GetString("cms.assets")

		if err := syncBanners(cms, assets); err != nil {
			panic(err)
		}
		fmt.Println("Banners synced")
		if err := syncPersons(cms, assets); err != nil {
			panic(err)
		}
		fmt.Println("Persons synced")
	},
}

func init() {
	RootCmd.AddCommand(cmsCmd)
}

func syncPersons(cms string, assets string) error {
	type PersonType struct {
		Id      int    `json:"id"`
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	// get persons as JSON
	url := cms + "get-posts/persons"

	client := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(res.Body)

	var persons []PersonType
	if err = json.Unmarshal(body, &persons); err != nil {
		return err
	}

	regLang, _ := regexp.Compile("persons-.+?-([a-z]{2})-html")

	for _, person := range persons {
		x := regLang.FindStringSubmatch(person.Slug)
		if len(x) == 0 {
			err = fmt.Errorf("person's slug MUST include language, but does not: %s", person.Slug)
			return err
		}

		// save
		m, err := json.Marshal(person)
		if err != nil {
			return err
		}
		err = os.MkdirAll(assets+"persons/", 0755)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(assets+"persons/"+person.Slug, m, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func syncBanners(cms string, assets string) error {
	type BannerType struct {
		Id      int                   `json:"id"`
		Slug    string                `json:"slug"`
		Title   string                `json:"title"`
		Content string                `json:"content"`
		Meta    []map[string][]string `json:"meta"`
	}

	regLang, _ := regexp.Compile(".+-([a-z]{2})")
	imgURL := viper.GetString("cms.image-url")
	regFindImage, _ := regexp.Compile("<img src=\"" + imgURL + "([^\"]+)\"")
	regSetImage, _ := regexp.Compile(imgURL)
	imgSrc := viper.GetString("cms.img-src")

	// get banners as JSON
	url := cms + "get-posts/banner"
	client := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(res.Body)

	var banners []BannerType
	if err = json.Unmarshal(body, &banners); err != nil {
		return err
	}
	for _, banner := range banners {
		x := regLang.FindStringSubmatch(banner.Slug)
		if len(x) == 0 {
			return fmt.Errorf("banner's slug MUST include language, but does not: %s", banner.Slug)
		}
		lang := x[1]
		x = regFindImage.FindStringSubmatch(banner.Content)
		if len(x) == 0 {
			return fmt.Errorf("banner (lang: %s) MUST include image, but does not: %s", lang, banner.Content)
		}
		img := x[1]

		// convert images' urls
		banner.Content = regSetImage.ReplaceAllString(banner.Content, imgSrc)

		// save
		m, err := json.Marshal(banner)
		if err != nil {
			return err
		}
		err = os.MkdirAll(assets+"banners/", 0755)
		if err != nil {
			return err
		}
		if err = ioutil.WriteFile(assets+"banners/"+banner.Slug, m, 0644); err != nil {
			return err
		}

		// create directories for images
		if err = os.MkdirAll(assets+"images/"+path.Dir(img), 0755); err != nil {
			return err
		}

		// copy images
		resp, err := http.Get(imgURL + img)
		if err != nil {
			return err
		}
		out, err := os.Create(assets + "images/" + img)
		if err != nil {
			return err
		}

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
		if err = resp.Body.Close(); err != nil {
			return err
		}
		if err = out.Close(); err != nil {
			return err
		}
	}

	return nil
}
