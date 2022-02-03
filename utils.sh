#!/usr/bin/env bash
build() {
    docker build -t hireviassh .
}

run-local() {
    docker run -p 23234:23234 -e PASSWORD="$1"  hireviassh 
}

connect() {
ssh -o "StrictHostKeyChecking=no" 127.0.0.1 -p 23234
}

build_and_run(){
    build
    run-local $1
}