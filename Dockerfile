FROM golang:1.26.4-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260518@sha256:6238b34be12469f667e1ccf93e03079e24f5d2f7c27e65452a32cb5709b2c429
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
