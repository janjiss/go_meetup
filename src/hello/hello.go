package hello

import (
	"appengine"
	"appengine/blobstore"
	"appengine/channel"
	// "appengine/datastore"
	"appengine/image"
	"appengine/taskqueue"
	"appengine/urlfetch"
	// "appengine/user"
	// "crypto/rand"
	// "encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"net/url"
	"regexp"
	"strings"

// "time"
)

var templates *template.Template = nil

func init() {
	http.HandleFunc("/", index)
	http.HandleFunc("/start", start)
	http.HandleFunc("/fetch", fetch)
}

func index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/start.html", http.StatusTemporaryRedirect)
}

func start(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	c := appengine.NewContext(r)

	client := urlfetch.Client(c)
	resp, err := client.Get("http://" + target)
	if error3(err, c, w) {
		return
	}

	len := int(resp.ContentLength)
	buf := make([]byte, len)
	read, err := resp.Body.Read(buf)
	if error3(err, c, w) {
		return
	}
	if read != len {
		http.Error(w, fmt.Sprintf("Target page Content-Length is %v but read %v bytes", len, read), http.StatusInternalServerError)
		return
	}

	rx, _ := regexp.Compile("<img .*? src=\"(.*?)\"")
	images := rx.FindAllSubmatch(buf, len)
	if images == nil {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "HTTP GET returned status %v\nNo images found\n\n", resp.Status)
		w.Write(buf)
		return
	}

	for _, image := range images {

		addr := string(image[1])
		// c.Infof("THIS IS IMAGE URL %v", addr)
		if strings.Index(addr, "http") == 0 {
			task := taskqueue.NewPOSTTask("/fetch", url.Values{"image": {addr}})
			_, err := taskqueue.Add(c, task, "default")
			if error3(err, c, w) {
				return
			}

		}
	}

	tok, err := channel.Create(c, "qwerty")
	if error3(err, c, w) {
		return
	}

	if templates == nil {
		var err error
		templates, err = template.ParseFiles("templates/gallery.html")
		if error3(err, c, w) {
			return
		}
	}

	w.Header().Set("Content-Type", "text/html")
	templates.ExecuteTemplate(w, "gallery.html", tok)

}

func fetch(w http.ResponseWriter, r *http.Request) {

	c := appengine.NewContext(r)

	imageUrl := r.FormValue("image")
	c.Infof("THIS IS IMAGE URL %v", imageUrl)
	client := urlfetch.Client(c)
	resp, err := client.Get(imageUrl)
	if error3(err, c, w) {
		return
	}

	blob, err := blobstore.Create(c, resp.Header.Get("Content-Type"))
	if error3(err, c, w) {
		return
	}
	written, err := io.Copy(blob, resp.Body)
	if error3(err, c, w) {
		return
	}
	if written < 100 {
		c.Infof("image is too small %v", written)
		return
	}
	err = blob.Close()
	if error3(err, c, w) {
		return
	}

	blobkey, err := blob.Key()
	if error3(err, c, w) {
		return
	}

	thumbnailUrl, err := image.ServingURL(c, blobkey, &image.ServingURLOptions{Size: 100})
	if error3(err, c, w) {
		return
	}
	t := thumbnailUrl.String()
	errr := channel.Send(c, "qwerty", t)
	if error3(errr, c, w) {
		return
	}
	// c.Infof("THIS IS IMAGE URL %v", t)
}

func error3(err error, c appengine.Context, w http.ResponseWriter) bool {
	if err != nil {
		msg := err.Error()
		c.Errorf("%v", msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return true
	}
	return false
}
