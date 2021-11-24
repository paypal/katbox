FROM alpine
LABEL maintainers="PayPal"
LABEL description="Katbox Driver"

# Add util-linux to get a new version of losetup.
RUN apk add util-linux
COPY ./bin/katbox-driver /katbox-driver
ENTRYPOINT ["/katbox-driver"]
