FROM golang:alpine as builder

RUN apk --no-cache add git ca-certificates

WORKDIR /build

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main -trimpath -ldflags "-w -s" .

FROM alpine:latest

RUN set -ex \
    &&  apk --no-cache add ca-certificates tzdata \
    &&  mkdir /server \
    &&  chown -R nobody /server

USER nobody
WORKDIR /server
COPY --from=builder /build/main .


EXPOSE 8080

CMD ["/server/main"]