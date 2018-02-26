package main

import (
	"net/http"

	"github.com/nomnom-ray/golang/webGCS/models"
	"github.com/nomnom-ray/golang/webGCS/router"
	"github.com/nomnom-ray/golang/webGCS/util"
)

func main() {

	models.Init()
	util.LoadTemplates("templates/*.html")

	r := router.LoadRoutes()
	http.Handle("/", r) //use the mux router as the default handler

	http.ListenAndServe(":8080", nil)
}
