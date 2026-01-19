FROM golang:1.25.6-alpine@sha256:d9b2e14101f27ec8d09674cd01186798d227bb0daec90e032aeb1cd22ac0f029
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
