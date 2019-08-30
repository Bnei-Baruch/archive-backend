package cms

import (
	"reflect"

	"github.com/pkg/errors"
)

type WPOptions struct {
	endpoint string
	username string
	password string
}

type WPRequest struct {
	username string
	password string
	auth bool
}

type WPApi struct {
	endpoint string
	WPRequest
}

func NewWPApi(options WPOptions) (api *WPApi, err error) {
	api = new(WPApi)
	v := reflect.TypeOf(options.endpoint)
	if v.Kind() != reflect.String {
		err = errors.Errorf("WPApi: options hash must contain an API endpoint URL string")
		return
	}
	// Ensure trailing slash on endpoint URI
	endpoint := options.endpoint
	last := endpoint[len(endpoint) - 1:]
	if last != "/" {
		endpoint += "/"
	}
	api.endpoint = endpoint
	if (options.username != "" ) || (options.password != "") {
		v := reflect.TypeOf(options.username)
		if v.Kind() == reflect.String {
			api.username = options.username
		}
		v = reflect.TypeOf(options.password)
		if v.Kind() == reflect.String {
			api.password = options.password
		}
		api.auth = true
	}
	return
}

func