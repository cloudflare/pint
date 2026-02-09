FROM golang:1.25.7-alpine@sha256:f6751d823c26342f9506c03797d2527668d095b0a15f1862cddb4d927a7a4ced
COPY . /src
WORKDIR /src
RUN apk add make git
RUN make

FROM debian:stable-20260202@sha256:3fd62b550de058baabdb17f225948208468766c59d19ef9a8796c000c7adc5a2
RUN apt-get update --yes && \
    apt-get install --no-install-recommends --yes git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=0 /src/pint /usr/local/bin/pint
WORKDIR /code
CMD ["/usr/local/bin/pint"]
