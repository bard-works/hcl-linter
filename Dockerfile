# Built by GoReleaser (dockers_v2), which places the binary for each platform
# under $TARGETPLATFORM/ in the build context.
#
# Alpine rather than distroless so CI systems that run a shell in the image
# (GitLab CI, Atlantis workflows) work out of the box.
FROM alpine:3.22

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/hcl-linter /usr/local/bin/hcl-linter

# Unprivileged by default. `check` only reads the mounted files; `fix` writes
# them, so run it as the files' owner:
#   docker run --rm -v "$PWD:/src" --user "$(id -u):$(id -g)" <image> fix .
RUN adduser -D -u 10001 linter
USER linter
WORKDIR /src

ENTRYPOINT ["hcl-linter"]
CMD ["--help"]
