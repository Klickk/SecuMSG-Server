# Bakes SQL into the image and uses migrate as entrypoint
FROM migrate/migrate:4
WORKDIR /
COPY . /migrations
ENTRYPOINT ["/migrate"]
