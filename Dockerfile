FROM golang:1.9-alpine

RUN  apk --no-cache add --virtual build-deps git

WORKDIR /go/src/github.com/j4y_funabashi/inari-admin
COPY pkg pkg/
COPY view view/
COPY main.go .

RUN go get ./...

EXPOSE 80
ENTRYPOINT [ "/go/bin/inari-admin" ]
