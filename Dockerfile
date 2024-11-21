FROM golang:alpine as build
WORKDIR /build

COPY go.sum go.mod /build/

RUN go mod download
RUN go mod tidy

COPY . /build/
RUN go build -o secret-sync

FROM ubuntu:latest

COPY --from=build /build/secret-sync /usr/local/bin/secret-sync
ENV PATH="/usr/local/bin:${PATH}"

ENTRYPOINT ["secret-sync"]
