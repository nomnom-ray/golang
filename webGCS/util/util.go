package util

import (
	"html/template"
	"net/http"
)

var templates *template.Template

//LoadTemplates initializes template
func LoadTemplates(htmlTemplates string) {
	templates = template.Must(template.ParseGlob(htmlTemplates))
}

//ExecuteTemplates passing data to html using updates; nil when nothing
func ExecuteTemplates(w http.ResponseWriter, htmlTemplates string, updates interface{}) {
	templates.ExecuteTemplate(w, htmlTemplates, updates)
}
