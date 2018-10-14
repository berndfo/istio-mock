package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"sync"
	"net/url"
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
	
	log.Printf("request received with URI %q", uri)

	if uri == "/health" {
		w.WriteHeader(http.StatusOK)
		return
	}

	uri = strings.TrimPrefix(uri, "/")
	
	executeCommand(uri, r)
	
	time.Sleep(time.Duration(300*time.Millisecond))
	
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
func executeCommand(uri string, originalRequest *http.Request) {
	if len(uri) == 0 {
		log.Println("no command")
		return
	}
	
	cmdRaw := uri 
	uriRemainder := "/"
	
	cmdEndIdx := strings.Index(uri, "/")
	if cmdEndIdx >= 0 {
		cmdRaw = uri[:cmdEndIdx] 
		uriRemainder = uri[cmdEndIdx:]
	}

	var err error
	cmdRaw, err = url.PathUnescape(cmdRaw)
	if err != nil {
		log.Printf("failed to unescape %q: %s", err)
	}
	
	log.Printf("received cmd %q, with uri remainder %q", cmdRaw, uriRemainder)
	
	if len(cmdRaw) == 0 {
		log.Printf("syntax error: command is empty")
		return
	}
	
	if strings.HasPrefix(cmdRaw, "@") && len(cmdRaw) > 1 {
		cmdForward := strings.TrimPrefix(cmdRaw, "@")
		executeParallelForwards(cmdForward, uriRemainder)
	} else {
		log.Printf("syntax error: unknown command %q", cmdRaw)
	}

}

func executeParallelForwards(cmdForward string, uriRemainder string) {
	parallelServices := strings.Split(cmdForward, "|")
	waitGroup := sync.WaitGroup{}
	for idx, forwardService := range parallelServices {
		waitGroup.Add(1)
		go func(idx int, forwardService string) {
			defer waitGroup.Done()

			forwardUrl := "http://" + forwardService + ":8080" + uriRemainder
			log.Printf("calling %d. service %q, url %q", idx+1, forwardService, forwardUrl)

			defer func() {
				if r := recover(); r != nil {
					log.Printf("calling %d. service %q failed with panic, url %q", idx+1, forwardService, forwardUrl)
				}
			}()

			resp, err := http.Get(forwardUrl)
			if err != nil {
				log.Printf("failure calling service %q, %q", forwardUrl, err)
			}
			log.Printf("success calling service %q, status = %s", forwardService, resp.Status)
		}(idx, forwardService)
	}
	waitGroup.Wait()
	log.Printf("all %d parallel forwards finished", len(parallelServices))
}

func main() {
	s := &http.Server{
		Addr:           ":8080",
		Handler:        &allHandler{},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Println("mock server listening on 8080")
	log.Fatal(s.ListenAndServe())

}
