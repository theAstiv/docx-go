# LibreOffice headless image used for the docx-go smoke test.
# Mounts a directory of .docx files and converts each to PDF; a non-zero
# exit from the soffice binary signals that LibreOffice could not open the
# file.  The PDF output is written to /out.
FROM debian:bookworm-slim

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
