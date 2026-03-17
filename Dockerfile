FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260316@sha256:37ad77961af28800b00f5f96a4dfda1855bdcf87f8431033f8b6a549d825bba3
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
