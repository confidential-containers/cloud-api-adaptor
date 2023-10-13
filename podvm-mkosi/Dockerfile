FROM debian:unstable-20230919-slim AS builder
RUN apt-get update && apt-get install -y \
	cpio \
	mkosi \
	mtools \
	zstd
RUN mkdir /mkosi
COPY mkosi.presets /mkosi/mkosi.presets
COPY mkosi.skeleton /mkosi/mkosi.skeleton
COPY mkosi.skeleton-debug /mkosi/mkosi.skeleton-debug
COPY mkosi.conf /mkosi/mkosi.conf
COPY resources /mkosi/resources
WORKDIR /mkosi
