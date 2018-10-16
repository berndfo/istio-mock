# this is a very simple container intended to act as a mock pod container 

it can be run locally

` export GOPATH=`pwd` `
` go run src/mockserver.go `

or as a docker container

` GOOS=linux go build -tags netgo -ldflags "-extldflags '-std++ -lm -static'" src/mockserver.go `
` mv mockserver docker/ `

` sudo docker build docker -t brainlounge/servicemock -t gcr.io/thelounge-lab/servicemock `

push to remote private container repo

` sudo gcloud docker -- push gcr.io/thelounge-lab/servicemock `

push to remote public container repo

` docker push brainlounge/servicemock `


# acquire cluster on Google Cloud

launching cluster on Google Kubernetes Engine, but any kubernetes cluster is sufficient

on GKE, fetch credentials, which needs the gcloud command line tool

` gcloud container clusters get-credentials mock-1 --region europe-west3-b `

grant local user full cluster access

` kubectl create clusterrolebinding cluster-admin-binding     --clusterrole=cluster-admin     --user=$(gcloud config get-value core/account) `

# deploy istio

download and install istio http://istio.io/

after extraction, in the istio base directory

` cd install/kubernetes `
` kubectl apply -f istio-demo.yaml `

wait for istio to come up (cancel the view with control-c)

` watch -n 0.5 kubectl -n istio-system get po,svc ` 

expose grafana tool, which is part of this istio deployment

` kubectl port-forward --namespace istio-system $(kubectl get pod --namespace istio-system --selector="app=grafana" --output jsonpath='{.items[0].metadata.name}') 8081:3000 & `

expose service graph, also part of this istio deployment

` kubectl port-forward --namespace istio-system $(kubectl get pod --namespace istio-system --selector="app=servicegraph" --output jsonpath='{.items[0].metadata.name}') 8082:8088 / `

these tools should now be available in your local browser via http://localhost:8081, http://localhost:8082/force/forcegraph.html

# create mock env within kubernetes aka new namespace
` kubectl create namespace mock-1 `

# make 
` kubectl config set-context $(kubectl config current-context) --namespace mock-1 `

# create the mock services you need

` sed 's/${istiomock}/service1/' kubernetes/mock-service-pod.yaml >kubernetes/service1-deployment.yaml `

repeat the above for more services, replace "service1" (2 times) with name of your service.

# make them istio-connected
for every service yaml created this way, run kube-inject:

` istioctl kube-inject -f kubernetes/service1-deployment.yaml >kubernetes/service1-deployment-with-sidecar.yaml `

# create mock service + pods
` kubectl apply -f kubernetes/service1-deployment-with-sidecar.yaml `

# make first service accessible from outside
` kubectl port-forward --namespace mock-1 $(kubectl get pod --namespace mock-1 --selector="app=service1" --output jsonpath='{.items[0].metadata.name}') 8080:8080 `

# example 

` http://localhost:8080/@service1/@service2/@service3/` `