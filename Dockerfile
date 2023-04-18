FROM golang:1.20 AS base
RUN apt-get update && apt-get -y upgrade && apt-get install -y libmagickwand-dev ffmpeg poppler-utils && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

FROM base AS builder
RUN mkdir /opt/gocacher
WORKDIR /opt/gocacher/

COPY go.mod /opt/gocacher/
COPY go.sum /opt/gocacher/
RUN go mod download

COPY ffmpeg/ /opt/gocacher/ffmpeg/
COPY imagick/ /opt/gocacher/imagick/
COPY pdf/ /opt/gocacher/pdf/
COPY processor/ /opt/gocacher/processor/
COPY raw/ /opt/gocacher/raw/
COPY main.go /opt/gocacher/
COPY types.go /opt/gocacher/

RUN go build -o gocacher && strip gocacher

FROM debian:bullseye-slim

RUN apt-get update && apt-get -y upgrade && apt-get install -y libmagickwand-6.q16-6 ffmpeg poppler-utils && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /opt/gocacher/gocacher /opt/gocacher
COPY config.yml /etc/gocacher/config.yml

CMD /opt/gocacher
