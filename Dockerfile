FROM golang:1.25.5-alpine@sha256:3587db7cc96576822c606d119729370dbf581931c5f43ac6d3fa03ab4ed85a10
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20251229@sha256:dff4def4601f20ccb9422ad7867772fbb13019fd186bbe59cd9fc28a82313283
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
