FROM golang:1.26.3-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d
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
