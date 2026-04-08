FROM golang:1.26.2-alpine@sha256:c2a1f7b2095d046ae14b286b18413a05bb82c9bca9b25fe7ff5efef0f0826166
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260406@sha256:ad4fc51a0bb73025eb126f1031ce41afe6b782a0a75fc296edc14ebf9f45abd2
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
