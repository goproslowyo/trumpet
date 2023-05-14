FROM cgr.dev/chainguard/python:latest-dev as builder
RUN pip install --report - -I --prefer-binary --compile --config-settings="--prefix=/" -U --ignore-installed yt-dlp

FROM cgr.dev/chainguard/python:latest
ENV PATH /home/nonroot/.local/bin/:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
COPY --from=builder --chown=65532:65532 /home/nonroot/.local /home/nonroot/.local
ENTRYPOINT [ "python" ]
