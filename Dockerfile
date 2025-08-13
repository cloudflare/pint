FROM golang:1.24.6-alpine
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20250811
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
