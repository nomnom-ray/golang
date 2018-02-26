package middleware

import (
	"net/http"

	"github.com/nomnom-ray/golang/webGCS/sessions"
)

//AuthRequired generates a session(connection to server) for the client; with optional login function
func AuthRequired(h http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		//middelware: the tasks of the middleware before reaching the handler "h" object to be executed
		session, _ := sessions.Store.Get(r, "session") //get session from redis
		_, ok := session.Values["user_id"]
		if !ok {
			http.Redirect(w, r, "/login", 302) //go to login page if no session
			return                             //return stops the process; doesn't proceed in main
		}

		//execute the handler
		h.ServeHTTP(w, r)
	}
}
