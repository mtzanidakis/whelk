FROM golang:1.27-alpine AS builder
RUN apk update && \
	apk add --no-cache ca-certificates tzdata && \
	update-ca-certificates
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o whelk ./cmd/whelk
RUN adduser \
	--disabled-password \
	--gecos "" \
	--home "/app" \
	--shell "/sbin/nologin" \
	--no-create-home \
	--uid 13128 \
	whelk
RUN egrep '^(whelk|root):' /etc/passwd > /etc/passwd.scratch && \
	egrep '^(whelk|root):' /etc/group > /etc/group.scratch

FROM scratch
WORKDIR /app
COPY --from=builder /etc/passwd.scratch /etc/passwd
COPY --from=builder /etc/group.scratch /etc/group
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/whelk /whelk

USER whelk
EXPOSE 3128
ENTRYPOINT ["/whelk"]
