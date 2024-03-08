# syntax=docker/dockerfile-upstream:1.5.0-rc2-labs
FROM lfedge/eve-alpine:9fb9b9cbf7d90066a70e4704d04a6fe248ff52bb AS build

ENV BUILD_PKGS curl build-base git ncurses-dev musl-dev python3
ENV PKGS alpine-baselayout ncurses-dev smartmontools bash mtools strace util-linux e2fsprogs e2fsprogs-extra jq

RUN eve-alpine-deploy.sh
ENV RUSTUP_HOME=/usr/local
ENV CARGO_HOME=/usr/local/cargo
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain none
ENV PATH "$PATH:/usr/local/cargo/bin"

WORKDIR /usr/local/my-installer
COPY . .
RUN cargo install --debug --path .

FROM scratch as image
COPY --from=build /out /
ENV RUST_BACKTRACE full
COPY --from=build /usr/local/cargo/bin/installer /sbin/installer
COPY run-installer.sh /sbin/run-installer.sh

WORKDIR /
ENTRYPOINT ["/sbin/run-installer.sh"]
