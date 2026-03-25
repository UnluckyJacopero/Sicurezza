//go:build !webui

package main

import (
	"net/http"
)

// registerWebUI è uno stub vuoto perché il tag `webui` non è stato specificato.
func registerWebUI(hdl http.Handler) (http.Handler, error) {
	return hdl, nil
}
