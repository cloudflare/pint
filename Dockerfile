FROM golang:1.26.2-alpine@sha256:c2a1f7b2095d046ae14b286b18413a05bb82c9bca9b25fe7ff5efef0f0826166
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260421@sha256:e969bc4a6af81e66bf2ee7e79cecb24885929111d5eddb97dd80bed3b90a5e93
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
