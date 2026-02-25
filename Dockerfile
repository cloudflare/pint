FROM golang:1.26.0-alpine@sha256:d4c4845f5d60c6a974c6000ce58ae079328d03ab7f721a0734277e69905473e5
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260223@sha256:46137948088890c3079c32df927b1aa59796192c7381501adcf90c15ee325382
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
