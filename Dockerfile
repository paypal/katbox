FROM alpine
LABEL maintainers="PayPal"
LABEL description="Katbox Driver"

# Add util-linux to get a new version of losetup.
RUN apk add util-linux
COPY ./bin/katboxplugin /katboxplugin
ENTRYPOINT ["/katboxplugin"]
