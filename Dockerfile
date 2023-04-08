FROM golang:alpine as builder

RUN apk --no-cache add git ca-certificates

WORKDIR /build

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main -trimpath -ldflags "-w -s" .

FROM alpine:latest

RUN set -ex \
    &&  apk --no-cache add ca-certificates tzdata \
    &&  mkdir /server \
    &&  mkdir /server/db\
    &&  adduser -H -D server\
    &&  chown -R server /server

USER server
WORKDIR /server
COPY --from=builder /build/main .

WORKDIR /server/db

EXPOSE 8080
VOLUME /server/db

CMD ["/server/main"]