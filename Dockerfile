FROM golang:1.25.5-alpine@sha256:3587db7cc96576822c606d119729370dbf581931c5f43ac6d3fa03ab4ed85a10
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260112@sha256:3b2f658b8c6ca18c5cf954161b0e7038e5d63f724914ef31641d1a5aedbde115
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
