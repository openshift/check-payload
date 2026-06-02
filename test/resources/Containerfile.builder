ARG BASE_IMAGE=registry.access.redhat.com/ubi9/go-toolset:1.26
FROM ${BASE_IMAGE}
USER root
ARG ZIG_VERSION=0.14.1
# ZIG_ARCH is set by build_fixture.sh from uname -m.
# No checksum pinning - zig publishes per-arch tarballs with different
# hashes, so a single pinned value would break multi-arch builds.
# TLS protects the download; this is a local dev builder, not production.
ARG ZIG_ARCH=x86_64
RUN dnf install -y --nodocs openssl-devel && dnf clean all \
    && curl -fSL -o /tmp/zig.tar.xz \
       "https://ziglang.org/download/${ZIG_VERSION}/zig-${ZIG_ARCH}-linux-${ZIG_VERSION}.tar.xz" \
    && tar -xJf /tmp/zig.tar.xz -C /usr/local \
    && ln -s /usr/local/zig-${ZIG_ARCH}-linux-${ZIG_VERSION}/zig /usr/local/bin/zig \
    && rm /tmp/zig.tar.xz
USER default
WORKDIR /build
