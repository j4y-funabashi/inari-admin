FROM golang:1.9

## install dep
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

WORKDIR /go/src/github.com/j4y_funabashi/inari-admin
COPY main.go Gopkg.lock Gopkg.toml ./
COPY pkg pkg/
COPY view view/

RUN dep ensure
RUN go install ./...

EXPOSE 80
ENTRYPOINT [ "/go/bin/inari-admin" ]
