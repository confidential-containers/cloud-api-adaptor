FROM rust:1.71.0-slim-bullseye as build
RUN apt-get update && apt-get install -y libssl-dev pkg-config protobuf-compiler curl gnupg2 build-essential clang
RUN curl -L https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add -
RUN echo 'deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu focal main' > /etc/apt/sources.list.d/intel-sgx.list
RUN apt-get update && apt-get install -y libtdx-attest-dev libtss2-dev
COPY Cargo.* /build/
COPY build.rs /build/
COPY protos /build/protos
COPY src /build/src
WORKDIR /build
RUN cargo b --release --bin skr-api

FROM debian:bullseye-slim as libraries
RUN apt-get update && apt-get install -y libtss2-tctildr0 libtss2-esys-3.0.2-0

FROM gcr.io/distroless/cc-debian11:latest
COPY --from=libraries /usr/lib/x86_64-linux-gnu/libtss2-* /usr/lib/x86_64-linux-gnu/
COPY --from=build /build/target/release/skr-api /skr-api
