# 使用 Go 官方环境
FROM golang:1.25

# 设置工作目录
WORKDIR /app

# 先拷贝依赖文件，下载依赖 (利用 Docker 缓存，速度快)
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源代码
COPY *.go ./

# 编译并运行
CMD ["go", "run", "main.go"]