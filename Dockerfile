FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /kodeku .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /kodeku /kodeku
ENV BYPASS_LOGIN=1
EXPOSE 8080
CMD ["/kodeku"]
