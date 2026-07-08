# DiClaw 内网服务器部署到 aiservice.byd.com

## 部署目标

本文档说明如何先在内网跑通：

```text
https://aiservice.byd.com/diclaw/
```

本阶段不考虑公网 `ids.byd.com` 代理。

目标链路：

```text
内网浏览器
  -> https://aiservice.byd.com/diclaw/
  -> Nginx 443
  -> http://127.0.0.1:18800/diclaw/
  -> PicoClaw Launcher
```

应用侧必须使用：

```bash
VITE_PUBLIC_BASE_PATH=/diclaw
PICOCLAW_PUBLIC_BASE_PATH=/diclaw
```

Nginx 侧必须保留 `/diclaw` 路径，不要 strip 掉。

## 1. 创建运行用户和目录

```bash
sudo useradd -r -m -d /var/lib/picoclaw -s /usr/sbin/nologin picoclaw || true

sudo mkdir -p /opt/picoclaw/bin
sudo mkdir -p /var/lib/picoclaw
sudo chown -R picoclaw:picoclaw /var/lib/picoclaw
```

目录用途：

```text
/opt/picoclaw/bin       存放 picoclaw 和 picoclaw-launcher 二进制
/var/lib/picoclaw       存放 config.json、launcher-config.json、日志、运行状态
```

## 2. 构建带 `/diclaw` 前缀的 launcher

在项目根目录执行：

```bash
VITE_PUBLIC_BASE_PATH=/diclaw make -C web build
```

确认生成：

```bash
ls -lh web/build/picoclaw-launcher
ls -lh build/picoclaw
```

如果 `build/picoclaw` 不存在，需要先按项目主程序构建流程生成它。

复制二进制：

```bash
sudo cp web/build/picoclaw-launcher /opt/picoclaw/bin/
sudo cp build/picoclaw /opt/picoclaw/bin/
sudo chmod +x /opt/picoclaw/bin/picoclaw-launcher /opt/picoclaw/bin/picoclaw
sudo chown root:root /opt/picoclaw/bin/picoclaw-launcher /opt/picoclaw/bin/picoclaw
```

## 3. 准备配置

如果已有配置，把它放到：

```text
/var/lib/picoclaw/config.json
```

确保权限：

```bash
sudo chown -R picoclaw:picoclaw /var/lib/picoclaw
```

## 4. 配置 systemd 服务

创建：

```bash
sudo tee /etc/systemd/system/picoclaw-launcher.service >/dev/null <<'EOF'
[Unit]
Description=PicoClaw Launcher
After=network.target

[Service]
User=picoclaw
Group=picoclaw
WorkingDirectory=/var/lib/picoclaw

Environment=PICOCLAW_HOME=/var/lib/picoclaw
Environment=PICOCLAW_BINARY=/opt/picoclaw/bin/picoclaw
Environment=PICOCLAW_PUBLIC_BASE_PATH=/diclaw

ExecStart=/opt/picoclaw/bin/picoclaw-launcher -no-browser -host 127.0.0.1 -port 18800
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now picoclaw-launcher
```

查看状态：

```bash
sudo systemctl status picoclaw-launcher
sudo journalctl -u picoclaw-launcher -f
```

## 5. 验证本机服务

在服务器上执行：

```bash
curl -i http://127.0.0.1:18800/diclaw/mobile
curl -i http://127.0.0.1:18800/diclaw/api/auth/status
```

预期：

```text
/diclaw/mobile -> 302 /diclaw/launcher-login?redirect=...
/diclaw/api/auth/status -> 200 JSON
```

如果这里失败，先不要配置 Nginx，优先检查：

```bash
sudo systemctl status picoclaw-launcher
sudo journalctl -u picoclaw-launcher -n 200
```

## 6. 安装并配置 Nginx

安装 Nginx：

```bash
sudo apt install nginx
```

RHEL/CentOS 类系统可使用：

```bash
sudo yum install nginx
```

准备证书文件，例如：

```text
/etc/nginx/ssl/aiservice.byd.com.crt
/etc/nginx/ssl/aiservice.byd.com.key
```

新增 Nginx 配置：

```bash
sudo tee /etc/nginx/conf.d/picoclaw-diclaw.conf >/dev/null <<'EOF'
map $http_upgrade $connection_upgrade {
    default upgrade;
    "" close;
}

server {
    listen 443 ssl http2;
    server_name aiservice.byd.com;

    ssl_certificate     /etc/nginx/ssl/aiservice.byd.com.crt;
    ssl_certificate_key /etc/nginx/ssl/aiservice.byd.com.key;

    location = /diclaw {
        return 301 /diclaw/;
    }

    location = /diclaw/ {
        return 302 /diclaw/mobile;
    }

    location ^~ /diclaw/ {
        proxy_pass http://127.0.0.1:18800;

        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-Port 443;
        proxy_set_header X-Forwarded-Prefix /diclaw;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600s;
    }
}
EOF
```

关键点：

- `proxy_pass` 必须写成 `http://127.0.0.1:18800`。
- 不要写成 `http://127.0.0.1:18800/`。
- 这样 Nginx 会把 `/diclaw/...` 原样转发给后端。
- 后端通过 `PICOCLAW_PUBLIC_BASE_PATH=/diclaw` 识别并剥离前缀。

测试并重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

## 7. 验证内网域名

在内网电脑或服务器上执行：

```bash
curl -k -I https://aiservice.byd.com/diclaw/
curl -k -I https://aiservice.byd.com/diclaw/mobile
curl -k -i https://aiservice.byd.com/diclaw/api/auth/status
```

预期：

```text
https://aiservice.byd.com/diclaw/       -> 302 /diclaw/mobile
https://aiservice.byd.com/diclaw/mobile -> 302 /diclaw/launcher-login?redirect=...
/diclaw/api/auth/status                 -> 200 JSON
```

验证登录页静态资源：

```bash
curl -k https://aiservice.byd.com/diclaw/launcher-login | grep /diclaw/assets
```

预期可以看到：

```text
/diclaw/assets/index-...
/diclaw/assets/...css
```

## 8. 验证 WebSocket

执行：

```bash
curl -k -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \
  -H "Sec-WebSocket-Version: 13" \
  https://aiservice.byd.com/diclaw/pico/ws
```

未登录时预期：

```text
HTTP/1.1 401 Unauthorized
unauthorized
```

这说明 Nginx 已经把 WebSocket Upgrade 请求转到了应用层。

如果返回下面状态，需要排查 Nginx 或上游网络：

```text
403
404
502
504
HTML 登录页
```

## 9. 常见问题

### 502 Bad Gateway

检查 launcher：

```bash
sudo systemctl status picoclaw-launcher
curl -i http://127.0.0.1:18800/diclaw/api/auth/status
```

### 静态资源 404

确认构建时带了：

```bash
VITE_PUBLIC_BASE_PATH=/diclaw
```

重新构建并部署：

```bash
VITE_PUBLIC_BASE_PATH=/diclaw make -C web build
sudo cp web/build/picoclaw-launcher /opt/picoclaw/bin/
sudo systemctl restart picoclaw-launcher
```

### 登录后仍然跳登录页

确认服务启动时带了：

```bash
PICOCLAW_PUBLIC_BASE_PATH=/diclaw
```

然后查看 Cookie Path 是否为：

```text
Path=/diclaw
```

### WebSocket 失败

确认 Nginx 配置有：

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection $connection_upgrade;
proxy_read_timeout 3600s;
```

### `/diclaw/` 是否一定跳 `/diclaw/mobile`

如果只提供移动端入口，保留：

```nginx
location = /diclaw/ {
    return 302 /diclaw/mobile;
}
```

如果希望电脑访问 `/diclaw/` 进入桌面管理 UI，则删除这段，让 `/diclaw/` 直接代理到后端：

```nginx
location ^~ /diclaw/ {
    proxy_pass http://127.0.0.1:18800;
    ...
}
```

移动端仍然可以直接访问：

```text
https://aiservice.byd.com/diclaw/mobile
```
