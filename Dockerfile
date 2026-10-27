FROM golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20261020@sha256:defb09e17b8c7326f9a3b63ecf0797f34674691f5c270d63987b955ff0168bd8
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
