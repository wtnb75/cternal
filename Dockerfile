FROM gcr.io/distroless/static:nonroot
COPY cternal /cternal
EXPOSE 8080
USER root
ENTRYPOINT ["/cternal", "serve"]
