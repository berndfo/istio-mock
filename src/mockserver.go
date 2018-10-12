package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type allHandler struct{}

type responseInfo struct {
	ReceivedDate string
	Message      string
	RequestInfo  []string
}

// formatRequest generates ascii representation of a request
func formatRequest(r *http.Request) []string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return request
}

func (h *allHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uri := r.RequestURI

	if uri == "/health" {
		w.WriteHeader(http.StatusOK)
		return
	}

	responseMessage := fmt.Sprintf("request '%v' succeeded.", uri)

	response := responseInfo{
		ReceivedDate: time.Now().Format(time.RFC3339Nano),
		Message:      responseMessage,
		RequestInfo:  formatRequest(r),
	}

	b, err := json.Marshal(response)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(b)
	w.WriteHeader(http.StatusOK)
}

func main() {
	s := &http.Server{
		Addr:           ":8080",
		Handler:        &allHandler{},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())

}
