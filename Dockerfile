FROM gcr.io/distroless/static:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/cternal /cternal
EXPOSE 8080
USER root
ENTRYPOINT ["/cternal"]
