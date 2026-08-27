FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260803@sha256:a317324860a60f88f98be05d1cab92f2262ef03884d1a6d7894894732ac9eb42
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
