FROM golang:1.26.2-alpine@sha256:27f829349da645e287cb195a9921c106fc224eeebbdc33aeb0f4fca2382befa6
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
