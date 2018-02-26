package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nomnom-ray/golang/webGCS/middleware"
	"github.com/nomnom-ray/golang/webGCS/models"
	"github.com/nomnom-ray/golang/webGCS/sessions"
	"github.com/nomnom-ray/golang/webGCS/util"
)

//LoadRoutes has r object repalcing http with router to serve complex multi user environments
func LoadRoutes() *mux.Router {
	r := mux.NewRouter()

	//different handles for different tasks
	r.HandleFunc("/", middleware.AuthRequired(indexGetHandler)).Methods("GET")   //for client to get stuff
	r.HandleFunc("/", middleware.AuthRequired(indexPostHandler)).Methods("POST") //for client to post stuff
	r.HandleFunc("/login", loginGetHandler).Methods("GET")                       //creates a page called "/login"
	r.HandleFunc("/login", loginPostHandler).Methods("POST")                     //create new session from a page called "/login"
	r.HandleFunc("/logout", logoutGetHandler).Methods("GET")
	r.HandleFunc("/register", registerGetHandler).Methods("GET") // uses seperate page for registeration
	r.HandleFunc("/register", registerPostHandler).Methods("POST")

	//for serving from static files. e.g. to serve css
	fs := http.FileServer(http.Dir("./static"))                        //inst. a file server object; and where files are served from
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs)) //tell routor to use path with static prefix

	r.HandleFunc("/{username}", //put these handlers with variable path names after static ones
		middleware.AuthRequired(userGetHandler)).Methods("GET")

	return r
}

//********************INDEX********************
func indexGetHandler(w http.ResponseWriter, r *http.Request) {

	updates, err := models.GetGlobalUpdates()
	if err != nil {
		util.InternalServerError(w)
		return
	}

	// util.ExecuteTemplates(w, "index.html", updates)
	util.ExecuteTemplates(w, "index.html", struct { //annonomous struct with field declarations
		Title       string
		Updates     []*models.Update
		DisplayForm bool
	}{
		Title:       "All updates",
		Updates:     updates,
		DisplayForm: true,
	})
}

func indexPostHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := sessions.Store.Get(r, "session")
	userIDInterface := session.Values["user_id"]
	userIDInSession, ok := userIDInterface.(int64)
	if !ok {
		util.InternalServerError(w)
		return
	}

	r.ParseForm()
	content := r.PostForm.Get("update") //from the comment tag in HTML
	err := models.PostUpdates(userIDInSession, content)
	if err != nil {
		util.InternalServerError(w)
		return
	}
	http.Redirect(w, r, "/", 302) //send client back to orginal submission page
}

//********************LOGIN********************

func loginGetHandler(w http.ResponseWriter, r *http.Request) {
	util.ExecuteTemplates(w, "login.html", nil) //passing data to html using comments; nil when nothing
}

func loginPostHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	user, err := models.LoginUser(username, password)
	if err != nil {
		switch err {
		case models.ErrUserNotFound:
			util.ExecuteTemplates(w, "login.html", "unknown user")
		case models.ErrInvalidLogin:
			util.ExecuteTemplates(w, "login.html", "bad login")
		default: //server error
			util.InternalServerError(w)

		}
		return //when all error is already handled
	}
	userID, err := user.GetID() //use userID instead of username to create session
	if err != nil {
		util.InternalServerError(w)

		return
	}
	session, _ := sessions.Store.Get(r, "session") //assigns cookied session to client request; create new if none
	session.Values["user_id"] = userID             //store and save session data
	session.Save(r, w)
	http.Redirect(w, r, "/", 302)
}

//********************LOGOUT********************
func logoutGetHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := sessions.Store.Get(r, "session")
	delete(session.Values, "user_id")
	session.Save(r, w)
	http.Redirect(w, r, "/Login", 302)
	// util.ExecuteTemplates(w, "login.html", nil) //passing data to html using comments; nil when nothing
}

//********************REGISTER********************

func registerGetHandler(w http.ResponseWriter, r *http.Request) {

	util.ExecuteTemplates(w, "register.html", nil) //passing data to html using comments; nil when nothing

}

func registerPostHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	err := models.RegisterUser(username, password)
	if err == models.UserExists {
		util.ExecuteTemplates(w, "register.html", "username taken")
	} else if err != nil {
		util.InternalServerError(w)

		return
	}
	http.Redirect(w, r, "/login", 302)
}

func userGetHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := sessions.Store.Get(r, "session")
	userIDInterface := session.Values["user_id"]
	userIDInSession, ok := userIDInterface.(int64)
	if !ok {
		util.InternalServerError(w)
		return
	}

	varsURL := mux.Vars(r) //get the variable portion of the URL path: map between URL and content
	username := varsURL["username"]

	user, err := models.GetUserbyName(username)
	if err != nil { //should be another 404 error to catch wrong user in the path
		util.InternalServerError(w)

		return
	}
	userID, err := user.GetID() //userID for the variable handler {username}
	if err != nil {
		util.InternalServerError(w)

		return
	}
	updates, err := models.GetUserUpdates(userID)
	if err != nil {
		util.InternalServerError(w)

		return
	}

	// util.ExecuteTemplates(w, "index.html", updates)

	util.ExecuteTemplates(w, "index.html", struct {
		Title       string
		Updates     []*models.Update
		DisplayForm bool
	}{
		Title:       username,
		Updates:     updates,
		DisplayForm: userID == userIDInSession,
	})

}
