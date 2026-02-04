FROM golang:1.26rc3-alpine@sha256:343c20fd6876bfb5ba9f46b0a452008b7dced3804e424ff7ada0ceadafad5c55
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
