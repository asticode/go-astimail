package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"text/template"
	"time"

	"github.com/asticode/go-astilog"
	"github.com/asticode/go-astimail"
	"github.com/asticode/go-astitools/template"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

var templates *template.Template

func serve(addr, pathResources string) (err error) {
	// Parse templates
	if templates, err = astitemplate.ParseDirectory(filepath.Join(pathResources, "templates"), ".html"); err != nil {
		return
	}

	// Build router
	var r = httprouter.New()
	r.GET("/", handleHomepage)
	r.POST("/encrypted", handleEncryptedMessages)
	r.POST("/users", handleCreateUser)
	r.GET("/validate_email/:token", handleValidateEmail)
	r.ServeFiles("/static/*filepath", http.Dir(filepath.Join(pathResources, "static")))

	// Listen
	astilog.Debugf("Listening on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, r); err != nil {
			astilog.Error(err)
		}
	}()
	return
}

func executeTemplate(rw http.ResponseWriter, name string, data interface{}) {
	// Check if template exists
	var t *template.Template
	if t = templates.Lookup(name); t == nil {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	// Execute template
	if err := t.Execute(rw, data); err != nil {
		astilog.Errorf("%s while handling homepage", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleHomepage(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	executeTemplate(rw, "/index.html", nil)
}

func handleCreateUser(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// Decode body
	var b astimail.BodyKey
	var err error
	if err = json.NewDecoder(r.Body).Decode(&b); err != nil {
		astilog.Errorf("%s while decoding body", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Generate server private key
	// TODO Use passphrase?
	var srvPrvKey *astimail.PrivateKey
	if srvPrvKey, err = astimail.GeneratePrivateKey(""); err != nil {
		astilog.Errorf("%s while generating server private key", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Fetch user
	if _, err = storage.UserFetchWithKey(b.Key); err != nil && err != errNotFound {
		astilog.Errorf("%s while fetching user", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// User already exists
	if err == nil {
		astilog.Error("User already exists")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create user
	if err = storage.UserCreate(b.Key, srvPrvKey); err != nil {
		astilog.Errorf("%s while creating user", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write
	if err = json.NewEncoder(rw).Encode(astimail.BodyKey{Key: srvPrvKey.Public()}); err != nil {
		astilog.Errorf("%s while writing", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleValidateEmail(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// Fetch email
	var e *Email
	var err error
	if e, err = storage.EmailFetchWithValidationToken(p.ByName("token")); err != nil && err != errNotFound {
		astilog.Errorf("%s while fetching email", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Validate email
	if err == nil {
		if err = storage.EmailValidate(e); err != nil {
			astilog.Errorf("%s while validating email", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// Execute template
	executeTemplate(rw, "/email_validated.html", nil)
}

func handleEncryptedMessages(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// Decode body
	var b astimail.BodyMessage
	var err error
	if err = json.NewDecoder(r.Body).Decode(&b); err != nil {
		astilog.Errorf("%s while decoding body", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Fetch user based on the key
	var u *User
	if u, err = storage.UserFetchWithKey(b.Key); err != nil {
		astilog.Errorf("%s while fetching user", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Switch on name
	switch b.Name {
	case astimail.NameEmailCreate:
		err = handleEmailCreate(b, u)
	case astimail.NameEmailFetch:
		err = handleEmailFetch(b, u)
	default:
		err = errors.New("Unknown b.Name")
	}

	// Process error
	if err != nil {
		astilog.Errorf("%s while handling %s", b.Name)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func handleEmailCreate(b astimail.BodyMessage, u *User) (err error) {
	// Decrypt message
	var email string
	if err = b.Decrypt(&email, u.ServerPrivateKey, u.ClientPublicKey, time.Now()); err != nil {
		err = errors.Wrap(err, "decrypting message failed")
		return
	}

	// Fetch user based on the email
	if _, err = storage.UserFetchWithEmail(email); err != nil && err != errNotFound {
		err = errors.Wrap(err, "fetching email failed")
		return
	}

	// Email already exists
	if err == nil {
		err = errors.New("Email already exists")
		return
	}

	// Create email
	var token string
	if token, err = storage.EmailCreate(email, u); err != nil {
		err = errors.Wrap(err, "creating email failed")
		return
	}
	astilog.Debugf("Token is %s", token)

	// TODO Send validation link
	return
}

func handleEmailFetch(b astimail.BodyMessage, u *User) (err error) {
	// Decrypt message
	var email string
	if err = b.Decrypt(&email, u.ServerPrivateKey, u.ClientPublicKey, time.Now()); err != nil {
		err = errors.Wrap(err, "decrypting message failed")
		return
	}

	// Fetch user based on the email
	if _, err = storage.UserFetchWithEmail(email); err != nil && err != errNotFound {
		err = errors.Wrap(err, "fetching email failed")
		return
	}
	return
}