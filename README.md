this is a very simple container intended to act as a mock pod container 

sudo docker build docker -t brainlounge/istiomock

export GOPATH=`pwd`
go run src/mock.go
