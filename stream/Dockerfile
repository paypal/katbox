#Build from golang alpine as base image and add build base 
FROM golang:1.16.0-alpine3.13 as golangbase
RUN apk add build-base

LABEL maintainer="The Katbox Authors"

WORKDIR /app
# Copy the source from the current directory to the Working Directory inside the container
COPY . .
RUN go mod download

RUN go build -o main .

#Start a new stage from scratch (multistage build)
FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=golangbase /app/main .
#TODO:@revanth  this port needs to updated before deploying on the cluster if needed
EXPOSE 5051
CMD ["./main"]
