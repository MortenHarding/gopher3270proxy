FROM golang:alpine

EXPOSE 7070/tcp

RUN apk --update add git \
&& git clone https://github.com/MortenHarding/gopher3270proxy.git \
&& cd gopher3270proxy \
&& go mod init gopher3270proxy \
&& go build -o /go/gopher3270proxy .

WORKDIR /go

ENTRYPOINT ["/go/gopher3270proxy/gopher3270proxy"]