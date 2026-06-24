FROM golang:1.26.4-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260623@sha256:9631e4628fccfb6f1ff9e27de2af0e82f61591c78d1584c778f92db9a541a3cc
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
