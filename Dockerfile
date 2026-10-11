FROM golang:1.27.1-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20261005@sha256:f7c6c9e4aba843dc41bac68bbf104773f71fabf53f74f8441049aaa56e50c4e2
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
