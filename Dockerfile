FROM golang:1.26.4-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260610@sha256:fa0ca9c113cbc97c1e2eb40d7012b43bdfb70abe4218229c366de911c5b32cd2
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
