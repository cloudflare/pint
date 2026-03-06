FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039
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
