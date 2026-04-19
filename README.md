# Telegram Music Bot (Go)

一个可部署到服务器的 Telegram Bot，支持：

- 指令搜索歌曲
- 选择歌曲并下载到服务器本地目录
- Docker / Docker Compose 部署

## 功能

- `/search 关键词`：搜索歌曲，点击后可继续选择下载音质
- `/download 歌曲ID`：按歌曲 ID 用默认音质直接下载
- `/quality [128|192|320|999]`：查看或设置默认下载音质
- `/path`：快捷选择保存目录（默认/标准/无损）
- `/setpath 目录`：设置自定义保存目录（支持相对/绝对路径）
- `/where`：查看当前会话保存目录
- 下载文件落地到服务器本地目录（默认 `./data/downloads`）
- 当网易云直链不可用时，自动回退到聚合音源（kuwo / joox / netease）继续尝试下载

## 运行前提

- 方式一：安装 Go 1.23+
- 方式二：安装 Docker 与 Docker Compose

## 参考项目思路

- 命令路由 + 回调按钮交互：借鉴 Music163bot-Go 的 Bot 交互方式
- 搜索与下载链路分离：借鉴 Solara 的 provider 设计思想

## 环境变量

复制示例配置：

```bash
cp .env.example .env
cp config/config.yaml.example config/config.yaml
cp secrets/telegram-bot-token.example secrets/telegram-bot-token
chmod 600 secrets/telegram-bot-token
```

编辑 `config/config.yaml`：

- `bot_token_file`：Bot Token 文件路径，建议使用 `./secrets/telegram-bot-token`
- `download_dir`：服务器本地下载目录
- `max_results`：搜索返回数量
- `http_timeout_seconds`：HTTP 超时秒数
- `http_max_retries`：搜索重试次数
- `source_api_base_url`：聚合音源接口地址，默认是 `https://music-api.gdstudio.xyz/api.php`
- `source_order`：搜索与回退下载的源顺序，例如 `netease -> kuwo -> joox`

编辑 `.env`（可选覆盖）：

- `CONFIG_FILE`：配置文件路径，默认 `./config/config.yaml`
- `BOT_TOKEN_FILE`：Token 文件路径，默认 `./secrets/telegram-bot-token`
- `DOWNLOAD_DIR`：覆盖下载目录
- `SOURCE_API_BASE_URL`：覆盖聚合音源接口地址
- `SOURCE_ORDER`：覆盖源顺序，逗号分隔，例如 `netease,kuwo,joox`

## 本地运行

```bash
make tidy
make test
make run
```

## Docker 部署

```bash
docker compose build
docker compose up -d
```

下载内容会保存到宿主机目录：

- `./data/downloads`

## Debian 13 服务器部署

### 一键部署（推荐）

如果代码已在服务器本机目录：

```bash
sudo bash deploy/debian13-oneclick.sh
```

如果要让脚本自动拉取仓库：

```bash
sudo bash deploy/debian13-oneclick.sh https://github.com/your-account/your-repo.git
```

执行后将自动完成：

- 安装依赖（git/golang/make）
- 编译 bot
- 安装 systemd 服务
- 启动服务 `music-bot.service`

首次部署后，编辑配置：

```bash
sudo nano /etc/music-bot/music-bot.env
sudo systemctl restart music-bot.service
```

### 方案 A: 原生 systemd（推荐）

1. 安装依赖（在 Debian 13 服务器）

```bash
sudo apt update
sudo apt install -y git golang make
```

2. 拉取代码并编译

```bash
git clone <your-repo-url> /opt/music-bot-src
cd /opt/music-bot-src
go mod tidy
go test ./...
go build -o music-bot ./cmd/bot
```

3. 执行引导脚本（自动创建用户、目录、systemd 服务）

```bash
sudo bash deploy/debian13-bootstrap.sh
```

4. 编辑环境变量

```bash
sudo nano /etc/music-bot/music-bot.env
```

至少修改：

- `TELEGRAM_BOT_TOKEN`

5. 重启并查看日志

```bash
sudo systemctl restart music-bot.service
sudo systemctl status music-bot.service
sudo journalctl -u music-bot.service -f
```

### 方案 B: Docker Compose

Debian 13 安装 Docker 后执行：

```bash
cp .env.example .env
cp config/config.yaml.example config/config.yaml
cp secrets/telegram-bot-token.example secrets/telegram-bot-token
chmod 600 secrets/telegram-bot-token
docker compose build
docker compose up -d
docker compose logs -f
```

Docker Compose 会自动挂载：

- `./config/config.yaml` 到容器内 `/app/config/config.yaml`
- `./secrets/telegram-bot-token` 到容器内 `/run/secrets/telegram-bot-token`

## 说明

- 音源下载可用性可能受版权与地区限制影响
- 若部分歌曲无法下载，属于上游音源限制

## 鸣谢与引用

本项目的设计与实现参考了以下两个项目：

- [akudamatata/Solara](https://github.com/akudamatata/Solara)
- [XiaoMengXinX/Music163bot-Go](https://github.com/XiaoMengXinX/Music163bot-Go)

主要借鉴内容包括：

- Solara 的多音源聚合、搜索/播放/下载链路设计，以及可配置源顺序的思路
- Music163bot-Go 的 Telegram Bot 命令分发、回调交互与下载流程组织方式

如果你喜欢这个项目，也建议直接支持原始项目与其作者。
