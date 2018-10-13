# this is a very simple container intended to act as a mock pod container 

export GOPATH=`pwd`
go run src/mockserver.go

GOOS=linux go build -tags netgo -ldflags "-extldflags '-std++ -lm -static'" src/mockserver.go
mv mockserver docker/

sudo docker build docker -t brainlounge/istiomock -t gcr.io/thelounge-lab/istiomock
sudo gcloud docker -- push gcr.io/thelounge-lab/istiomock

istioctl kube-inject -f kubernetes/mock-pod.yaml >kubernetes/mock-pod-with-sidecar.yaml

# deploy cluster

... launch cluster ...
on GKE, fetch credentials
> gcloud container clusters get-credentials mock-1 --region europe-west3-b

> kubectl create clusterrolebinding cluster-admin-binding     --clusterrole=cluster-admin     --user=$(gcloud config get-value core/account)

# install istio
> cd $ISTIO
> cd install/kubernetes
> kubectl apply -f istio-demo.yaml

# create mock env 
kubectl create ns mock-1

kubectl config set-context $(kubectl config current-context) --namespace mock-1

