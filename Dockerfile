FROM golang:1.27.4-alpine@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20261123@sha256:a32b8984739287d78c4783b0825bdaaad187d157b16a98e0c0c5e5f2675d81fa
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
