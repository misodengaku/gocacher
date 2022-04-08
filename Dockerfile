FROM golang:1.18
RUN apt-get update && apt-get -y upgrade && apt-get install -y libmagickwand-dev ffmpeg poppler-utils && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
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


RUN go build

COPY config.yml /etc/gocacher/config.yml

CMD /opt/gocacher/gocacher
