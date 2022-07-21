# Use an intermediate container for initial building
FROM golang:1.18-buster AS builder
RUN apt-get update && apt-get install -y xz-utils upx ca-certificates youtube-dl --no-install-recommends && apt-get clean && rm -rf /var/lib/apt/lists/*

# Let go packages call C code
ENV GO111MODULE=on CGO_ENABLED=1 GOAMD64=v3
WORKDIR /build
COPY src .
RUN GOOS=linux GOARCH=amd64 go build -a -v -ldflags="-extldflags '-static' -s -w" -tags 'osusergo,netgo,static' -asmflags 'all=-trimpath={{.Env.GOPATH}}' .

# Install static ffmpeg
RUN curl -LO https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz && \
    tar -xJf ffmpeg-release-amd64-static.tar.xz && \
    mv ffmpeg-5.*-amd64-static/ffmpeg /build && \
    rm -rf ffmpeg-*-static

# Compress the binary and verify the output using UPX
# h/t @FiloSottile/Filippo Valsorda: https://blog.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/
# RUN upx -kv --ultra-brute /build/ffmpeg
# RUN upx -kv --ultra-brute /build/trumpet

# Copy the contents of /dist to the root of a scratch containter
FROM python:slim
RUN apt-get update && apt-get install -y youtube-dl --no-install-recommends && apt-get clean && rm -rf /var/lib/apt/lists/*
COPY --chown=1000:1000 --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --chown=1000:1000 --from=builder /build/trumpet /
COPY --chown=1000:1000 --from=builder /build/ffmpeg /usr/bin
# RUN mkdir /audio && chown 1000:1000 /audio
USER 1000
WORKDIR /
ENTRYPOINT ["/trumpet"]
