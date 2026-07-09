FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2
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
