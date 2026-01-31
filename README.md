# dnspod-updater

一个可容器化部署的小工具：

- 运行在 Docker `--network host` 模式下
- 自动获取宿主机“默认路由对应的局域网网卡”的 IPv4 地址
- 使用 DNSPod 传统 API（Token）调用 `Record.Info` + `Record.Modify` 更新解析记录
- 启动时执行一次；可按环境变量设置定期检查，IP 变化才会触发更新（避免“无变动修改”导致锁定）

## 快速开始（Docker）

构建：

```bash
docker build -t dnspod-updater:latest .
```

运行（Linux 才支持真正的 `--network host`）：

```bash
docker run --rm --network host \
 -e DNSPOD_LOGIN_TOKEN="ID,Token" \
 -e DNSPOD_DOMAIN="example.com" \
 -e DNSPOD_SUB_DOMAIN="www" \
 -e CHECK_INTERVAL="5m" \
 dnspod-updater:latest
```

## 快速开始（docker-compose + .env）

1) 复制配置文件：

```bash
cp .env.example .env
```

1) 编辑 `.env`，填入 `DNSPOD_LOGIN_TOKEN` / `DNSPOD_DOMAIN` / `DNSPOD_SUB_DOMAIN` 等（可选填 `DNSPOD_RECORD_ID`）。

2) 启动：

```bash
docker compose up -d
```

查看日志：

```bash
docker compose logs -f
```

只运行一次（启动即更新/不更新后退出）：

```bash
docker run --rm --network host \
 -e DNSPOD_LOGIN_TOKEN="ID,Token" \
 -e DNSPOD_DOMAIN="example.com" \
 -e DNSPOD_SUB_DOMAIN="www" \
 -e ONESHOT=true \
 dnspod-updater:latest
```

## 环境变量

### 必填

- `DNSPOD_LOGIN_TOKEN`：DNSPod Token，格式 `id,token`
- `DNSPOD_DOMAIN` 或 `DNSPOD_DOMAIN_ID`：二选一

以下二选一：

- `DNSPOD_RECORD_ID`：记录 ID
- `DNSPOD_SUB_DOMAIN`：主机记录（如 `www`），默认 `@`（当未指定 `DNSPOD_RECORD_ID` 时使用）

### 常用可选（记录参数）

- `DNSPOD_SUB_DOMAIN`：主机记录，默认 `@`
- `DNSPOD_RECORD_TYPE`：默认 `A`
- `DNSPOD_RECORD_LINE`：默认 `默认`
- `DNSPOD_RECORD_LINE_ID`：若填写则优先使用（例如 `10=0`）
- `DNSPOD_TTL`：TTL 秒数，默认不设置
- `DNSPOD_STATUS`：默认 `enable`
- `DNSPOD_WEIGHT`：0-100；不设置请留空（默认）

说明：

- 当未指定 `DNSPOD_RECORD_ID` 时，会调用 `Record.List` 按 `sub_domain` + `record_type`（默认 A）获取记录列表，并选择第一条记录作为要更新的记录。
- 如果你的同一个 `sub_domain` 下存在多条线路/多条同类型记录，建议直接配置 `DNSPOD_RECORD_ID`，或通过 `DNSPOD_RECORD_LINE_ID` 锁定线路。

### 定时与运行

- `CHECK_INTERVAL`：例如 `30s` / `5m` / `1h`；也支持纯数字（按秒）
- `CHECK_INTERVAL_SECONDS`：兼容字段，秒
- `ONESHOT`：`true` 表示只运行一次
- `START_DELAY`：启动延迟，例如 `10s`
- `HTTP_TIMEOUT`：例如 `10s`

### IP 探测

- `IP_DETECT_METHOD`：`auto`(默认) / `route` / `udp` / `iface`
- `IP_PREFERRED_IFACE`：指定网卡名（如 `eth0`），配合 `iface` 或作为优先项
- `WIFI_SSID`：可选；指定后仅当检测到“某个无线网卡正在连接该 SSID”时才会获取其 IPv4，否则会记录日志并跳过本轮更新

说明：

- `route`：Linux 下解析 `/proc/net/route` 找默认路由网卡，然后取该网卡 IPv4（推荐）
- `udp`：通过 UDP Dial 推断本机出站源地址

注意：

- `WIFI_SSID` 的实现需要在 Linux 上通过 netlink 读取当前关联的 WiFi 信息；在容器里可能需要额外权限（如 `--cap-add NET_ADMIN` 或 `privileged`），取决于宿主机内核/安全策略。

## 本地运行（Go）

```bash
go run ./cmd/dnspod-updater
```

## 注意事项

- DNSPod 传统 API 有“1 小时内超过 5 次无变动修改会锁定 1 小时”的限制；本工具会先 `Record.Info` 比较当前值，只有 IP 变化才调用 `Record.Modify`。
- `docker-compose.yml` 使用了 `network_mode: host`，这在 Linux 上最符合“拿宿主机默认路由网卡 IP”的需求；Docker Desktop（macOS/Windows）对 host 网络支持不同，可能无法达到预期。
