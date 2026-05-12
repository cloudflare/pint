FROM golang:1.26.3-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260505@sha256:23759e4f84483a4a2b312eb72e0aa68ad3526344fb2e7caa2eb64846b0d7e142
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
