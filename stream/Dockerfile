FROM alpine
LABEL maintainer="PayPal"
LABEL description="Katbox Stream"

RUN apk add util-linux
COPY ./bin/katbox-stream /katbox-stream
ENTRYPOINT ["/katbox-stream"]
