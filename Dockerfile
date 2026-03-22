# Stage 1: Build Tailwind CSS
FROM node:22-alpine AS css
WORKDIR /build
RUN npm install tailwindcss @tailwindcss/cli
COPY static/css/app.css static/css/app.css
COPY templates/ templates/
RUN npx @tailwindcss/cli -i static/css/app.css -o static/css/dist.css --minify

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS build
RUN go install github.com/a-h/templ/cmd/templ@latest
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN templ generate
COPY --from=css /build/static/css/dist.css static/css/dist.css
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /server .
COPY --from=build /build/static/js/ static/js/
COPY --from=css /build/static/css/dist.css static/css/dist.css
COPY migrations/ migrations/
RUN mkdir -p /app/data

ENV DB_PATH=/app/data/google-tasks.db
EXPOSE 8080
CMD ["/app/server"]
