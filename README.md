this is a very simple container intended to act as a mock pod container 

export GOPATH=`pwd`
go run src/mockserver.go

GOOS=linux go build -tags netgo -ldflags "-extldflags '-std++ -lm -static'" src/mockserver.go
mv mockserver docker/

sudo docker build docker -t brainlounge/istiomock -t gcr.io/thelounge-lab/istiomock
sudo gcloud docker -- push gcr.io/thelounge-lab/istiomock

istioctl kube-inject -f kubernetes/mock-pod.yaml >kubernetes/mock-pod-with-sidecar.yaml