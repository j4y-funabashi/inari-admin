FROM golang:1.9

WORKDIR /go/src/github.com/j4y_funabashi/mock-mpserver
COPY main.go home.html ./

RUN go get ./...
RUN go install ./...

EXPOSE 80
ENTRYPOINT [ "/go/bin/mock-mpserver" ]
