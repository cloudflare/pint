FROM golang:1.27.1-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260918@sha256:2bf225230f05881b6a36d20f484088f408c9a1309edc730c0bf01f15e635d411
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
