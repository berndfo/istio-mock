FROM golang:1.10

LABEL maintainer="bernd@brainlounge.de"

COPY ./mockserver /usr/local/bin/

EXPOSE 8080
ENTRYPOINT /usr/local/bin/mockserver