FROM golang:1.27.4-alpine@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf
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
