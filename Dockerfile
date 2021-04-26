FROM golang:1.16.3
COPY . /src
WORKDIR /src
RUN go build ./cmd/pint

FROM debian:stable
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
