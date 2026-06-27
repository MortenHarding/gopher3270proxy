FROM golang:alpine AS builder

RUN apk --no-cache add git \
 && git clone https://github.com/MortenHarding/gopher3270proxy.git /src

WORKDIR /src
RUN go mod init gopher3270proxy \
 && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gopher3270proxy .

FROM scratch

COPY --from=builder /gopher3270proxy /gopher3270proxy

EXPOSE 7070/tcp
WORKDIR /var/cnf
ENTRYPOINT ["/gopher3270proxy"]