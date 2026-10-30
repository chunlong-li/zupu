# ===== 阶段1：构建后端 =====
FROM golang:alpine AS backend
ENV GOPROXY=https://goproxy.cn,direct
# 安装 gcc 以支持 CGO（sqlite3 需要）
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY backend/ ./
RUN go mod tidy && go mod download && \
    CGO_ENABLED=1 go build -o familytree .

# ===== 阶段2：运行时 =====
FROM alpine:3.19
# 启用 community 仓库（ffmpeg 需要）
RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata curl zip ffmpeg

# 缓存破坏：每次构建时 docker-compose.yml 里改 CACHEBUST 数字即可
ARG CACHEBUST=1
RUN echo "Cache bust: ${CACHEBUST}"

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -sf /usr/share/zoneinfo/$TZ /etc/localtime && echo "$TZ" > /etc/timezone
# 注：定时备份统一由 Go 后端内置调度器负责（每日 02:00 日备份、每月1日 03:00 月备份），
# 不再使用容器 cron + /app/backup.sh，避免双重备份产生重复文件

WORKDIR /app
COPY --from=backend /app/familytree .
# 直接使用宿主预编译的前端 dist
COPY frontend/dist ./web

# 备份脚本（保留 daily/monthly 命令兼容，供手动调用）
RUN printf '#!/bin/sh\nTYPE=${1:-daily}\nBACKUP_DIR=/data/backup\nDATE=$(date +%%Y%%m%%d_%%H%%M%%S)\nmkdir -p $BACKUP_DIR/$TYPE\ncd /data && zip -rq $BACKUP_DIR/$TYPE/familytree_${TYPE}_$DATE.zip data/ upload/\nif [ "$TYPE" = "daily" ]; then\n  find $BACKUP_DIR/$TYPE -name "familytree_daily_*.zip" -mtime +30 -delete\nfi\n' > /app/backup.sh && \
    chmod +x /app/backup.sh

ENV FAMILY_TREE_DATA_DIR=/data \
    FAMILY_TREE_PORT=8080 \
    FAMILY_TREE_TIMEZONE=Asia/Shanghai

VOLUME ["/data"]
EXPOSE 8080

# 启动脚本：优先使用数据卷 /data/runtime 中的二进制和前端（支持在线更新覆盖后自重启），
# 否则回退到镜像内置的 /app 版本
RUN printf '#!/bin/sh\nRUNTIME=/data/runtime\nif [ -x $RUNTIME/familytree ]; then\n  cp -f $RUNTIME/familytree /app/familytree\n  chmod +x /app/familytree\nfi\nif [ -d $RUNTIME/web ]; then\n  rm -rf /app/web\n  cp -r $RUNTIME/web /app/web\nfi\n' > /app/load_runtime.sh && chmod +x /app/load_runtime.sh

# 启动主服务（不再启动 crond，定时备份由 Go 调度器内置）
CMD sh -c 'sh /app/load_runtime.sh && exec ./familytree'
