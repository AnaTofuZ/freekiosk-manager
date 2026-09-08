package web

import (
	"encoding/json"
	"net/http"
	"strings"

	assets "github.com/AnaTofuZ/freekiosk-manager/web"
)

var english = loadLanguage("en")
var japanese = loadLanguage("ja")

func loadLanguage(lang string) map[string]string {
	data, err := assets.Files.ReadFile("locales/" + lang + ".json")
	if err != nil {
		panic(err)
	}
	var text map[string]string
	if err := json.Unmarshal(data, &text); err != nil {
		panic(err)
	}
	return text
}

func language(r *http.Request) string {
	if lang := r.URL.Query().Get("lang"); lang == "en" || lang == "ja" {
		return lang
	}
	if cookie, err := r.Cookie("language"); err == nil && cookie.Value == "ja" {
		return "ja"
	}
	return "en"
}

func translations(lang string) map[string]string {
	if lang == "ja" {
		return japanese
	}
	return english
}

func translate(lang, message string) string {
	if lang == "ja" {
		for key, value := range english {
			if value == message {
				return japanese[key]
			}
		}
		if strings.HasPrefix(message, "Device returned HTTP ") {
			return "HTTPエラー (" + strings.TrimPrefix(message, "Device returned HTTP ") + ")"
		}
	}
	return message
}
