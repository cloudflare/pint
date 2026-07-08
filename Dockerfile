FROM golang:1.27rc2-alpine@sha256:7870fdc211100210e7380f487953c4188fcbeac99646a56926a973161a3eedcd
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
