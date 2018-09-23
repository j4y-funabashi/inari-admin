#!/bin/sh

VERSION=0.0.8
PROFILE=j4y
S3_BUCKET=dep-inari-admin

build_lambda () {
    echo $1
    echo "deploying version $VERSION"
    mkdir -p bin/$1
    env GOOS=linux go build -ldflags="-s -w" -o bin/$1/function fn/$1/main.go;
    chmod a+x bin/$1/function
    rm function.zip
    zip -jr function.zip bin/$1/function
    zip -ur function.zip view/
    aws --profile=$PROFILE s3 cp function.zip s3://$S3_BUCKET/$1/$VERSION/function.zip
    rm function.zip
}

build_lambda login
build_lambda login-init
build_lambda login-callback
