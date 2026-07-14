FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260713@sha256:8e109a974a9659354791cab2c001e5c3e3153805c344ccec7c1ef98d814187e7
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
