# Build the linux/amd64 binary on the host, then copy it into a distroless image.
# Usage: make docker-build (from this directory)
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY bin/inspiration-linux /app/inspiration
COPY --chown=nonroot:nonroot migrations/ /app/migrations/
EXPOSE 8210
USER nonroot:nonroot
ENTRYPOINT ["/app/inspiration"]
CMD ["serve"]
