# Use an intermediate container for initial building
FROM golang:bullseye AS builder
RUN apt-get update && apt-get install -y upx ca-certificates --no-install-recommends && apt-get clean && rm -rf /var/lib/apt/lists/*

# Let go packages call C code
ENV GO111MODULE=on CGO_ENABLED=1 GOAMD64=v3
WORKDIR /build
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -a -v -ldflags="-extldflags '-static' -s -w" -tags 'osusergo,netgo,static' -asmflags 'all=-trimpath={{.Env.GOPATH}}' .

# Compress the binary and verify the output using UPX
# h/t @FiloSottile/Filippo Valsorda: https://blog.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/
RUN upx -v --lzma --best /build/trumpet

# Copy our binaries to root of yt-dlp chainguard container
FROM ghcr.io/goproslowyo/chainguard-python-yt-dlp:latest
COPY --chown=65532:65532 --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --chown=65532:65532 --from=ghcr.io/goproslowyo/ffmpeg-static:latest /ffmpeg /usr/bin/ffmpeg
COPY --chown=65532:65532 --from=builder /build/trumpet /usr/bin/trumpet
USER nonroot
WORKDIR /trumpet
ENTRYPOINT ["trumpet"]
LABEL org.opencontainers.image.authors='goproslowyo@gmail.com'
LABEL org.opencontainers.image.description="Trumpet"
LABEL org.opencontainers.image.licenses='GPL-3.0'
LABEL org.opencontainers.image.source='https://github.com/goproslowyo/trumpet'
LABEL org.opencontainers.image.url='https://github.com/users/goproslowyo/packages/container/package/trumpet'
LABEL org.opencontainers.image.vendor='GoProSlowYo'
