FROM rust:1.71.0-slim-bullseye as build
RUN apt-get update && apt-get install -y libssl-dev pkg-config protobuf-compiler curl gnupg2 build-essential clang
COPY Cargo.* /build/
COPY attester /build/attester
COPY skr-api /build/skr-api
COPY protos /build/protos
WORKDIR /build
RUN cargo b --release -p skr-api

FROM gcr.io/distroless/cc-debian11:nonroot
COPY --from=build /build/target/release/skr-api /skr-api
