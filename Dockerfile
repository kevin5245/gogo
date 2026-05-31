# 第一阶段：编译 Go 程序
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 禁用 CGO，编译纯静态二进制文件
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

COPY go.mod ./
COPY main.go ./

# 编译并去除调试信息减小体积
RUN go build -ldflags="-s -w" -o scraper-server main.go

# 第二阶段：极简运行环境
FROM alpine:latest

# 安装 HTTPS 请求必备的根证书，以及中国上海时区
RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Shanghai
# Render 会自动注入 PORT 环境变量，这里作为保底默认值
ENV PORT=8000 

WORKDIR /app

# 把第一阶段编译好的程序复制过来
COPY --from=builder /app/scraper-server .

EXPOSE $PORT

# 启动服务
CMD ["./scraper-server"]
