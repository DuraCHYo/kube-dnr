# Образ собирается GoReleaser (dockers_v2): бинарник для каждой платформы
# уже лежит в контексте сборки в каталоге $TARGETPLATFORM.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/kube-dnr /usr/local/bin/kube-dnr

ENV HOME=/home/nonroot
WORKDIR /tmp
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kube-dnr"]
