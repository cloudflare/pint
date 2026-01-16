FROM golang:1.26rc2-alpine@sha256:ebf4414ac1ebce5d728a1b9446037ba9aee1b46228bbc8972fb57819a91e164e
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260112@sha256:3b2f658b8c6ca18c5cf954161b0e7038e5d63f724914ef31641d1a5aedbde115
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
