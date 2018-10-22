package main

import (
	"fmt"
	"net/http"
	"strings"
	"log"
	"time"
	"encoding/json"
	"net/url"
	"sync"
	"bytes"
	"flag"
)

type allHandler struct{}

type ForwardInfo struct {
	Url string
	Result string
}

type ForwardInfos struct {
	SequentialForwards []ForwardInfo
}

type responseInfo struct {
	ReceivedDate     string
	Message          string
	ParallelForwards []ForwardInfos
	RequestInfo      []string
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

/*
 ServeHTTP handles a mock-server request. the URI determines if a forward cascade of services is called or the request
 is simply returned. the uri "/health" is an exception, and can be used as a health check for the service. 
 any other expression until the first "/" in the URI is interpreted as a command and handled by executeCommand()
 
 */
func (h *allHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uri := r.RequestURI

	log.Printf("request received with URI %q", uri)

	if uri == "/health" {
		w.WriteHeader(http.StatusOK)
		return
	}

	uri = strings.TrimPrefix(uri, "/")

	forwardInfos := executeCommand(uri, r)

	time.Sleep(time.Duration(300 * time.Millisecond))

	responseMessage := fmt.Sprintf("request '%v' succeeded.", uri)

	response := responseInfo{
		ReceivedDate:     time.Now().Format(time.RFC3339Nano),
		Message:          responseMessage,
		ParallelForwards: forwardInfos,
		RequestInfo:      formatRequest(r),
	}

	reponseJson, err := json.Marshal(response)

	var jsonIndented bytes.Buffer
	_ = json.Indent(&jsonIndented, reponseJson, "", "\t")
	reponseJson = jsonIndented.Bytes()

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(reponseJson)
}

func isCommand(uri string) bool {
	return strings.HasPrefix(uri, "@")
}

/*
 executeCommand treats everything until the first "/" in the uri as the command. the remainder is taken unchanged and in 
 most cases used as the new URI for forward service calls.
 currently, any other command not starting with "@" is not understood and treated as an error.

 commands starting with a "@" are treated as forward service calls. 
 this is how they work:
 everything after "@" is used to make the calls. the simplest case are commands like "@myservice" or "myservice:8080" 
 where on service is called. in both cases, a HTTP request to these destinations is made.

 however, to support sequential and parallel calls, pipes and commas are treated specially. 
 example: "@service1|service2" - service1 and service2 get called in parallel
 example: "@service1,service2" - service1 and service2 get called sequentially 

 any number of pipes and commas can be used. 
 for a mix of pipes and commas, pipes take precedence over commas. that means, all parallel calls are set up, with 
 sequences of services within these parallel flows.
 example: "@service1,service2|service3|service4,service5|" - service2 is executed after service1 returns. service5 is 
 executed after service4 returns. both sequences (1+2 and 4+5) together with a one-call sequence for service3 are 
 executed in parallel.  

 in all cases, all forward services are called with the remainder of the original URI as the new request URI.
 example: "@service1|service2,service3/newuriforallservices" leeds to 3 calls:
 1. "service1:8080/newuriforallservices"
 2.1. "service2:8080/newuriforallservices" 
 2.2. "service3:8080/newuriforallservices" 
 */
func executeCommand(uri string, originalRequest *http.Request) (forwards []ForwardInfos) {
	
	if len(uri) == 0 || uri == "/" {
		log.Printf("empty command in %q", uri)
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

	if !isCommand(cmdRaw) {
		log.Printf("no command in %q", cmdRaw)
		return
	}

	log.Printf("received cmd %q, with uri remainder %q", cmdRaw, uriRemainder)

	if len(cmdRaw) == 0 {
		log.Printf("syntax error: command is empty")
		return
	}

	if strings.HasPrefix(cmdRaw, "@") && len(cmdRaw) > 1 {
		cmdForward := strings.TrimPrefix(cmdRaw, "@")
		forwards = executeParallelForwards(cmdForward, uriRemainder)
	} else {
		log.Printf("syntax error: unknown command %q", cmdRaw)
	}
	return
}

func executeParallelForwards(cmdForward string, uriRemainder string) (parallelForwards []ForwardInfos) {
	parallelServices := strings.Split(cmdForward, "|")
	numParaForwards := len(parallelServices)
	log.Printf("calling %d services in parallel: %s", numParaForwards, strings.Join(parallelServices, ", "))

	parallelForwards = make([]ForwardInfos, 0)
	
	forwardReceiver := make(chan *ForwardInfos)
	go func() {
		// collect all results from parallel, concurrent calls
		for {
			select {
			case forwardsInfos := <-forwardReceiver:
				if forwardsInfos == nil {
					return
				} else {
					parallelForwards = append(parallelForwards, *forwardsInfos)
				}
			}
		}
	}()
	
	waitGroup := sync.WaitGroup{}
	// go through all parallel, concurrently called services...
	for idx, forwardServiceList := range parallelServices {
		waitGroup.Add(1)
		// ...and branch off a go func for each service.
		go func(idxParallel int, serviceList string) {
			defer waitGroup.Done()

			infos := ForwardInfos{}
			
			sequentialForwards := strings.Split(serviceList, ",")
			if len(sequentialForwards) > 1 {
				infos.SequentialForwards = executeSequentialForwards(sequentialForwards, idxParallel, uriRemainder)
			} else {
				idxDisplay := fmt.Sprintf("%d", (idxParallel + 1))
				info := executeForward(sequentialForwards[0], uriRemainder, idxDisplay)
				infos.SequentialForwards = []ForwardInfo {info}
			}
			forwardReceiver<-&infos
		}(idx, forwardServiceList)
	}
	waitGroup.Wait()
	forwardReceiver <- nil // end result-collecting process
	if numParaForwards > 1 {
		log.Printf("all %d parallel forwards finished", numParaForwards)
	}
	return
}

func executeSequentialForwards(sequentialForwards []string, idxParallel int, uriRemainder string) (infos []ForwardInfo) {
	numSeqForwards := len(sequentialForwards)
	infos = make([]ForwardInfo, numSeqForwards)
	log.Printf("calling %d services sequentially: %s", numSeqForwards, strings.Join(sequentialForwards, ", "))
	for idxSeq, forwardService := range sequentialForwards {
		idxDisplay := fmt.Sprintf("%d.%d", (idxParallel + 1), (idxSeq + 1))
		infos[idxSeq] = executeForward(forwardService, uriRemainder, idxDisplay)
	}
	log.Printf("all %d sequential forwards finished", numSeqForwards)
	return
}

func executeForward(forwardService string, uriRemainder string, indexDisplay string) (forwardInfo ForwardInfo) {
	if !strings.Contains(forwardService, ":") {
		forwardService = forwardService + ":8080"
	}
	if (!strings.HasPrefix(uriRemainder, "/")) {
		uriRemainder = "/" + uriRemainder
	}
	forwardUrl := "http://" + forwardService + uriRemainder
	forwardInfo.Url = forwardUrl
	log.Printf("calling %s. service %q, url %q", indexDisplay, forwardService, forwardUrl)
	defer func() {
		if r := recover(); r != nil {
			panicMsg := fmt.Sprintf("calling %s. service %q failed with panic, url %q", indexDisplay, forwardService, forwardUrl)
			forwardInfo.Result = panicMsg
			log.Printf(panicMsg)
		}
	}()
	resp, err := http.Get(forwardUrl)
	if err != nil {
		errMsg := fmt.Sprintf("failure calling service %q, %q", forwardUrl, err)
		log.Printf(errMsg)
		forwardInfo.Result = errMsg
	} else {
		msg := fmt.Sprintf("success calling service %q, status = %s", forwardService, resp.Status)
		log.Printf(msg)
		forwardInfo.Result = msg
	}
	return
}

func main() {

	var listenAddress = flag.String("listenAddress", "0.0.0.0:8080", "the listen address of this service instance. defaults to '0.0.0.0:8080', but for example ':8443' or '0.0.0.0:8081' are valid, too.")

	flag.Parse()
	
	s := &http.Server{
		Addr:           *listenAddress,
		Handler:        &allHandler{},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Printf("mock server listening on %q", (*listenAddress))
	log.Fatal(s.ListenAndServe())
}
