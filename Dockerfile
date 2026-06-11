FROM golang:1.26.4-alpine@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c
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
