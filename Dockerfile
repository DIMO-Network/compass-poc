FROM golang:1.23 AS build

RUN useradd -u 10001 dimo

WORKDIR /go/src/github.com/DIMO-Network/compass-poc
COPY . /go/src/github.com/DIMO-Network/compass-poc/

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOFLAGS=-mod=vendor

RUN ls
RUN go mod tidy
RUN go mod vendor
RUN make install

FROM busybox AS package

LABEL maintainer="DIMO <hello@dimo.zone>"

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/github.com/DIMO-Network/compass-poc/target/bin/stream .

USER dimo

EXPOSE 8888

CMD /stream
