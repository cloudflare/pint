FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83
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
