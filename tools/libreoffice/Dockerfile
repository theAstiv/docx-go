# LibreOffice headless image used for the docx-go smoke test.
# Mounts a directory of .docx files and converts each to PDF; a non-zero
# exit from the soffice binary signals that LibreOffice could not open the
# file.  The PDF output is written to /out.
# Pinned by digest (the tag is for readers); Dependabot proposes updates.
FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends libreoffice-writer ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# soffice insists on a writable HOME so it can create its profile.
ENV HOME=/tmp/lohome
RUN mkdir -p /tmp/lohome /files /out

WORKDIR /files
ENTRYPOINT ["soffice", "--headless", "--nologo", "--norestore", "--nofirststartwizard", \
            "--convert-to", "pdf", "--outdir", "/out"]
