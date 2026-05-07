FROM golang:1.26.3-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260421@sha256:e969bc4a6af81e66bf2ee7e79cecb24885929111d5eddb97dd80bed3b90a5e93
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
