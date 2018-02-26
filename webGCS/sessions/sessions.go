package sessions

import "github.com/gorilla/sessions"

//Store use cookies (from other ways) to store session of a user
var Store = sessions.NewCookieStore([]byte("T0p-s3cr3t")) //make unreproducible cookies: s3cr3t as encrption key
