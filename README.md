# 体育赛事直播源自动抓取 (Docker + Render + UptimeRobot)

这是一个基于 Go 编写的轻量级赛事直播源抓取工具，专门适配了免费白嫖 Render 容器服务的设计。

## 🎯 部署流转原理

1. 代码推送到 GitHub `main` 分支。
2. **GitHub Actions** 自动编译出最小化的 Docker 镜像，并发布到 GitHub Container Registry (GHCR)。
3. Action 在最后一步通过 Webhook 通知 **Render** 拉取最新镜像部署。
4. **UptimeRobot** 每隔 15~30 分钟访问一次 Render 的 `/trigger` 接口，唤醒服务并触发数据抓取。

## 🚀 部署步骤

### 步骤 1：上传代码并公开镜像
1. 将所有文件推送到你的 GitHub 仓库（主分支设为 `main`）。
2. 在 GitHub 个人主页 -> **Packages** 中找到生成的镜像，并将其可见性 (Visibility) 修改为 **Public**。

### 步骤 2：配置 Render Web Service
1. 登录 [Render](https://render.com/)，创建全新的 **Web Service**。
2. 选择部署现有的镜像 (`Deploy an existing image from a registry`)。
3. 填入你的镜像地址，例如 `ghcr.io/你的用户名/仓库名:latest`。
4. **无需配置环境变量**（Render 会自动分发 `$PORT` 变量给 Go 程序）。
5. 部署完成后，在 Render 的 "Settings" 中找到 **Deploy Hook** URL 并复制。

### 步骤 3：绑定自动触发器
1. 回到 GitHub 仓库 -> **Settings** -> **Secrets and variables** -> **Actions**。
2. 新建一个 Repository secret：
   - Name: `RENDER_DEPLOY_HOOK`
   - Secret: 粘贴刚才复制的 Render Deploy Hook。
3. 以后每次修改代码 `push` 后，Render 都会全自动更新！

### 步骤 4：配置 UptimeRobot (防休眠 & 定时抓取)
为了防止 Render 的免费容器休眠，同时也利用它作为外部定时器：
1. 注册 [UptimeRobot](https://uptimerobot.com/)。
2. 创建一个新的 HTTP 监控 (Monitor)：
   - URL: `https://<你的Render应用名>.onrender.com/trigger`
   - 间隔: **15 分钟**
3. 这样不仅让服务永不失联，还实现了每 15 分钟全自动更新一次最新的比赛源。

## 📥 获取接口
- **播放列表**: `https://<你的Render应用名>.onrender.com/playlist.m3u`
- **纯净文本**: `https://<你的Render应用名>.onrender.com/live_links.txt`
- **运行日志**: `https://<你的Render应用名>.onrender.com/scraper_log.txt`
