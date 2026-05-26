FROM gcr.io/distroless/static:nonroot
COPY cternal /cternal
EXPOSE 8080
ENTRYPOINT ["/cternal", "serve"]
