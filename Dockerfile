FROM golang:1.21-alpine3.18 AS builder
WORKDIR /build
COPY . /build/
RUN go build -o unik .

FROM alpine:3.18
RUN apk --no-cache upgrade && apk add --no-cache ca-certificates tzdata && update-ca-certificates && \
    rm -rf /var/cache/apk/* && \
  addgroup -S unik && adduser -S unik -G unik
USER unik
COPY --chown=unik:unik --chmod=0755 --from=builder /build/unik /usr/local/bin/unik
ENTRYPOINT ["/usr/local/bin/unik"]