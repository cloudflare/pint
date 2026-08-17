FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83
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
