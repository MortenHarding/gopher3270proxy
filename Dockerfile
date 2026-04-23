FROM golang:alpine

EXPOSE 7070/tcp

RUN apk --update add git \
&& git clone https://github.com/MortenHarding/gopher3270proxy.git \
&& cd gopher3270proxy \
&& go build -o /go/gopher3270proxy . \
&& cd /go \
&& rm -rf ./gopher3270proxy

WORKDIR /go

CMD ["./gopher3270proxy"]