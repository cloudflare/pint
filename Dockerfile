FROM golang:1.27.1-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260824@sha256:7fa362f5e7c036d7f45621a6c721c03493cdcb5889adfcd65587759e34fe4c6f
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
