FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20261104@sha256:3f0347f4b50e7a54a91d1beaf6ce6fa5aec392347da6a9184b5bfe4e1b814538
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
