FROM golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20261027@sha256:23ef8ad09b87c0dbee73f1f962ee78a6515842480a4444a2e5275912d4cad8e4
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
