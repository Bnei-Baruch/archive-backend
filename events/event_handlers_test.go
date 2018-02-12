package events

import (
	"testing"
	"net/http"
	log "github.com/Sirupsen/logrus"

)

func TestApiGet(t *testing.T){
	apiType := "unzip"
	uid := "ZLuOz4ih"
	apiURL := "https://archive.kbb1.com/assets/api/"
	resp, err := http.Get(apiURL + "/"+ apiType +"/" + uid)
	defer resp.Body.Close()
	if err != nil {
		log.Errorf("unzip failed: %+v", err)
	}
	if resp.StatusCode != 200 {
		log.Errorf("we got response %d for api unzip request. file UID is \"%s\"", resp.StatusCode, uid)
	}
	log.Infof("response status code for unzipping file \"%s\" is: %d", uid, resp.StatusCode)
}


