# Run mock services to create a service mesh

[Istio](istio.io) helps with meshing services on Kubernetes.
This tiny project provides a mock service called `mockserver` you can deploy multiple times to test it out. Using `mockserver`, you can create cascades of service calls easily just by typing an URL. The server is bundles as a Docker Container as [`brainlounge/servicemock`](https://hub.docker.com/r/brainlounge/servicemock/).

## Prerequisites

You should have basic understanding of the following technologies:
* Kubernetes incl. using kubectl, deploying pods, services, proxying service ports
* Setting up a Kubernetes cluster (Minikube, ECS, GKE etc.) 
* Docker (run containers, configure port forwards, kill containers)
* Shell (run commands, terminate processes, put processes to background, use `cURL`, use `sed`, optionally have `watch` installed)
* (optional) run a Go program, build etc. 


## Build from scratch

Please skip this section, if you simply want to run the mock service only on Kubernetes and Istio, and don't want to build or run the service locally.

### Run locally (no Docker, no Kubernetes) 

After cloning this git repo, the service can be run locally:

`` export GOPATH=`pwd` `` (optional for recent Go versions)

` go run src/mockserver.go -listenAddress localhost:8080`

To demonstrate the service mesh aspect, the service can be started multiple times. 
Locally, you can run multiple service instances by changing the listen port number:

` go run src/mockserver.go -listenAddress localhost:8081`

` go run src/mockserver.go -listenAddress localhost:8082`

### Build container image

` GOOS=linux go build -tags netgo -ldflags "-extldflags '-std++ -lm -static'" src/mockserver.go `

` mv mockserver docker/ `

` sudo docker build docker -t brainlounge/servicemock -t gcr.io/thelounge-lab/servicemock `

### Push to remote repository

Push to remote private container repo (GCP version)

` sudo gcloud docker -- push gcr.io/thelounge-lab/servicemock `

Push to remote public container repo

` docker push brainlounge/servicemock `

### Run Docker images locally

The container image can run directly:

` docker run -ti  -p8089:8080 brainlounge/servicemock `

The service is exposed locally on port 8089.
it will be fetched from Docker Hub, if not already present. 

## Mocking a Service Mesh on Kubernetes and Istio

Now we get to the interesting part. It shows how to create a set of services and call them.

### Acquire a Kubernetes cluster on Google Cloud

Launching cluster on Google Kubernetes Engine, but any Kubernetes cluster is sufficient.

On GKE, when the cluster is ready, fetch credentials, which needs the gcloud command line tool

` gcloud container clusters get-credentials mock-1 --region europe-west3-b `

Grant local user full cluster access on the cluster:

` kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value core/account) `

### Deploy Istio

[Download and install Istio](https://istio.io/docs/setup/kubernetes/download-release/).

After extraction, change into the Istio base directory, then deploy Istio into your Kubernetes cluster:

` cd install/kubernetes `

` kubectl apply -f istio-demo.yaml `

Wait for Istio to come up, it can take a while: 

` watch -n 0.5 kubectl -n istio-system get po,svc ` (cancel this view with control-c) 

Expose the Grafana tool, which is part of the Istio deployment

` kubectl port-forward --namespace istio-system $(kubectl get pod --namespace istio-system --selector="app=grafana" --output jsonpath='{.items[0].metadata.name}') 8091:3000 & `

Expose the Service Graph, which is also part of this Istio deployment:

` kubectl port-forward --namespace istio-system $(kubectl get pod --namespace istio-system --selector="app=servicegraph" --output jsonpath='{.items[0].metadata.name}') 8092:8088 & `

These tools should now be available in your local browser via http://localhost:8091 or http://localhost:8092/force/forcegraph.html respectively.

Both are interesting later when services get called.

### Prepare Kubernetes namespace

It's always a good idea to isolate units of work into separate namespaces.

Create mock env within kubernetes aka new namespace:

` kubectl create namespace mock-1 `

Make new namespace current, so we can omit explicit `--namespace` parameters for commands from now on.
  
` kubectl config set-context $(kubectl config current-context) --namespace mock-1 `

### Create the mock services you need

`mock-service-pod.yaml` is a template for creating any number of services. All those services differ _in name only_, their runtime is exactly the same container. This way, they are actually distinct deployments and can be addressed by other services by name. They can then be meshed together.

` sed 's/${istiomock}/service1/' kubernetes/mock-service-pod.yaml >kubernetes/service1-deployment.yaml `

Repeat the above for more services, replace "service1" (2 times) in the above command line with the name of your service.

Of course, you can adopt some parametrization in this file like the number of replicas.

### Make them Istio-ready
Services handled by Istio require special preparation. Therefore, for every service yaml created in the previous step, run kube-inject:

` istioctl kube-inject -f kubernetes/service1-deployment.yaml >kubernetes/service1-deployment-with-sidecar.yaml `

### Deploy mock service + pods

Now it's finally time to let the services loose on the cluster:

` kubectl apply -f kubernetes/service1-deployment-with-sidecar.yaml `

(!) Repeat this line for every service.
You should see the services initialize after some time.

You can watch this on the command line like this (needs installation of `watch` package):

` watch -n 0.5 kubectl get po,svc `

### Make entry service accessible from outside

To reach the services, at least one of them must be made available using a local port forward like this (example for namespace `mock-1`, service name `service1`):

` kubectl port-forward --namespace mock-1 $(kubectl get pod --namespace mock-1 --selector="app=service1" --output jsonpath='{.items[0].metadata.name}') 8080:8080 `

` curl localhost:8080/ `

should now return 

TODO 

### Calling the Mesh of Services

Services can now be called chained, in parallel or in sequence. Services can even call themselves.
Every reachable service can be used as a starting point. For the sake of simplicity, we assume the starting point is always reached at `localhost:8080` but it can be any other address where a mockserver is listening.

| URL | effect |
|---|---|
| `http://localhost:8080/service/calls/@service3/` | first service (reached at localhost:8080) is called, but nothing else happens (reason: no '@' directly after 'localhost:8080/' |
| `http://localhost:8080/@service1/the-end` | first service is called, which then calls 'service1' with URI '/the-end'. |
| `http://localhost:8080/@service1:8081/the-end` | first service is called, which then calls 'service1' on port 8081, with URI '/the-end'. |
| `http://localhost:8080/@service1/@service2/@service3/` | call chain: localhost -> service1 -> service2 -> service3 (all using port 8080)|
| `http://localhost:8080/@service1/service2` | call chain: localhost -> service1 (no '@' before 'service2' |
| `http://localhost:8080/@service1,service2/uri-reuse/` | service at localhost calls first service1, then service2, both with URI /uri-reuse/ |
| `http://localhost:8080/@service1\|service2/uri-reuse/` | service at localhost calls service1 and service2 in parallel, both with URI /uri-reuse/ |
| `http://localhost:8080/@service1\|service2/@service3\|service4/uri/` | service at localhost calls service1 and service2 in parallel, both call service3 and service4 in parallel |

## Take it away, Istio..

Services behavior can be controlled and configured using Istio. We built the services, now we can alter their behavior using Istio. Have fun!  


