FROM golang:1.16
RUN apt-get update && apt-get -y upgrade && apt-get install -y libmagickwand-dev ffmpeg && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir /opt/gocacher
# RUN sed -i -e 's%<policy domain="coder" rights="none" pattern="PDF" />%<!--<policy domain="coder" rights="none" pattern="PDF" />-->%' /etc/ImageMagick-6/policy.xml

COPY ffmpeg/ /opt/gocacher/ffmpeg/
COPY imagick/ /opt/gocacher/imagick/
COPY processor/ /opt/gocacher/processor/
COPY raw/ /opt/gocacher/raw/

COPY main.go /opt/gocacher/
COPY types.go /opt/gocacher/
COPY go.mod /opt/gocacher/


WORKDIR /opt/gocacher/
RUN go get && go build -o gocacher main.go types.go

COPY config.yml /etc/gocacher/config.yml

CMD /opt/gocacher/gocacher
