FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Shanghai

COPY bin/internal/people /app/

EXPOSE 50051

CMD ["/app/people"]