FROM golang:1.14-alpine as builder
RUN apk add --no-cache -q binutils
WORKDIR /app
ADD ./ /app
RUN CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -a -o /entrypoint . && \
    strip /entrypoint

FROM alpine:3
RUN apk add --no-cache -q git
COPY --from="builder" /entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
