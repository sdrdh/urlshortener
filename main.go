package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	uri  = flag.String("d", os.Getenv("DB_URI"), "Mongo Database to Use")
	port = flag.String("p", os.Getenv("PORT"), "Port to run the service on")
)

const (
	//DB is the name of the database
	DB = "urls"
	//COLLECTION is the name of the collection
	COLLECTION = "urls"
)

var session *mgo.Session

var errInvalidID = fmt.Errorf("Invalid ID")

type shortURL struct {
	ID       string      `json:"id" bson:"_id"`
	URL      string      `json:"url" bson:"url"`
	HitCount int         `json:"hitCount" bson:"hitCount,omitempty"`
	Metadata interface{} `json:"meta" bson:"meta,omitempty"`
}

type shortenRequest struct {
	URL      string      `json:"url"`
	Metadata interface{} `json:"meta"`
}

func writeResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func loggingMiddleware(methods []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timerFunc := func(start time.Time) {
			log.Printf("Request: %s %s took %s", r.Method, r.RequestURI, time.Since(start))
		}
		defer timerFunc(time.Now())
		for _, method := range methods {
			if method == r.Method {
				next(w, r)
				return
			}
		}
		writeResponse(w, http.StatusMethodNotAllowed, nil)
	}
}

func getURIDetailsByID(id string) (*shortURL, error) {
	if len(id) != 8 {
		return nil, errInvalidID
	}
	s := session.Clone()
	defer s.Close()
	var su shortURL
	err := s.DB(DB).C(COLLECTION).Find(bson.M{"_id": id}).One(&su)
	if err != nil {
		return nil, err
	}
	return &su, nil
}

func getURIDetailsByURL(url string) (*shortURL, error) {
	s := session.Clone()
	defer s.Close()
	var su shortURL
	err := s.DB(DB).C(COLLECTION).Find(bson.M{"url": url}).One(&su)
	if err != nil {
		return nil, err
	}
	return &su, nil
}

func insert(obj *shortURL) error {
	s := session.Clone()
	defer s.Close()
	return s.DB(DB).C(COLLECTION).Insert(obj)
}

func update(obj *shortURL) error {
	s := session.Clone()
	defer s.Close()
	return s.DB(DB).C(COLLECTION).UpdateId(obj.ID, obj)
}

func ensureIndexes() error {
	log.Println("Creating Indexes")
	s := session.Clone()
	defer s.Close()
	index := mgo.Index{Key: []string{"url"}}
	return s.DB(DB).C(COLLECTION).EnsureIndex(index)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.RequestURI[1:]
	ud, err := getURIDetailsByID(uri)
	if err != nil {
		writeResponse(w, http.StatusNotFound, nil)
		return
	}
	ud.HitCount++
	update(ud)
	log.Printf("Redirecting to: %s\n", ud.URL)
	http.Redirect(w, r, ud.URL, http.StatusTemporaryRedirect)
}

func getMD5(url string) string {
	uh := md5.Sum([]byte(url))
	us := hex.EncodeToString(uh[:])
	return us
}

func urlDetailsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.Form.Get("shortId")
		ud, err := getURIDetailsByID(id)
		if err == errInvalidID {
			writeResponse(w, http.StatusBadRequest, nil)
			return
		}
		if err != nil {
			writeResponse(w, http.StatusNotFound, nil)
			return
		}
		writeResponse(w, http.StatusOK, ud)
		return
	case http.MethodPost:
		var sq shortenRequest
		err := json.NewDecoder(r.Body).Decode(&sq)
		if err != nil {
			writeResponse(w, http.StatusBadRequest, nil)
			return
		}
		ud, err := getURIDetailsByURL(sq.URL)
		if err == nil {
			writeResponse(w, http.StatusOK, ud)
			return
		}
		su := &shortURL{
			URL:      sq.URL,
			Metadata: sq.Metadata,
		}
		mdHash := getMD5(sq.URL)
		for i := 0; i <= len(mdHash)-8; i++ {
			su.ID = mdHash[i : i+8]
			err := insert(su)
			if err == nil {
				log.Printf("No of db calls: %d\n", i+1)
				break
			}
		}
		writeResponse(w, http.StatusOK, su)
	}
}

func getHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/shorten", loggingMiddleware([]string{http.MethodGet, http.MethodPost}, urlDetailsHandler))
	mux.HandleFunc("/", loggingMiddleware([]string{http.MethodGet}, redirectHandler))
	return mux
}

func main() {
	flag.Parse()
	var err error
	if *port == "" || *uri == "" {
		panic("Either port or db uri is missing")
	}
	session, err = mgo.Dial(*uri)
	if err != nil {
		panic(err)
	}
	err = ensureIndexes()
	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", *port),
		Handler:           getHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
	}
	log.Println("Starting the server")
	log.Fatal(server.ListenAndServe())
}
