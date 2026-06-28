# GLM Token 计费分支部署说明

本文说明 `glm-token-billing-multiplier` 分支的两种部署方式：

- 另一台 PC 本地 Docker Compose 测试
- 生产服务器构建本地镜像并替换应用服务

该分支包含 `glm-5.1` / `glm-5.2` 的可配置 token 计费倍率逻辑。命中倍率后，服务端使用日志、扣费统计以及返回给下游客户端的 OpenAI/OpenAI Responses `usage` 会使用调整后的 token 数量。

注意：缓存读取 token 不参与倍率档位判断，也不会被倍率放大。倍率档位按“非缓存输入 token + 输出 token”判断，避免短输入请求因为命中大量缓存而直接进入高倍率档位。

## 方案一：另一台 PC 本地测试

本地测试时要从当前源码构建镜像，不要直接使用官方镜像。

### 1. 拉取分支

```bash
git clone -b glm-token-billing-multiplier https://github.com/LightGao-Hub/new-api.git
cd new-api
```

### 2. 准备倍率配置

该步骤可选，但建议保留配置文件，后续修改倍率时不需要重新编译镜像。

Linux/macOS:

```bash
mkdir -p data
cp config/token_billing_tiers.example.json data/token_billing_tiers.json
```

Windows PowerShell:

```powershell
mkdir data
copy config\token_billing_tiers.example.json data\token_billing_tiers.json
```

Docker 容器内工作目录是 `/data`，默认会读取：

```text
/data/token_billing_tiers.json
```

如果该文件不存在或解析失败，会使用代码内置默认倍率。

### 3. 使用本地源码构建并启动

不要只执行：

```bash
docker compose up -d
```

默认 `docker-compose.yml` 中的 `new-api` 服务使用的是官方镜像：

```yaml
image: calciumion/new-api:latest
```

应该执行：

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

含义：

- `docker-compose.yml` 提供 Redis、PostgreSQL、端口、挂载目录等基础配置
- `docker-compose.local.yml` 覆盖 `new-api` 服务，让它从当前源码目录构建镜像
- `--build` 表示启动前重新构建镜像
- `-d` 表示后台运行

`docker-compose.local.yml` 中的关键覆盖配置是：

```yaml
services:
  new-api:
    build:
      context: .
      dockerfile: Dockerfile
    image: new-api:glm-token-local
```

### 4. 访问和验证

浏览器访问：

```text
http://localhost:3000
```

查看当前镜像：

```bash
docker images
docker ps
```

应能看到本地构建镜像：

```text
new-api:glm-token-local
```

如果只看到 `calciumion/new-api:latest`，说明没有使用本地源码构建。

## 方案二：生产服务器部署

生产目录通常只包含 compose 配置、数据目录、日志目录、PostgreSQL 和 Redis 数据目录，不一定是源码目录。例如：

```text
/root/new-api
  data/
  logs/
  postgres/
  redis/
  docker-compose.yml
```

不要把 GitHub 源码直接 clone 到这个生产目录里覆盖文件。推荐源码单独放一个目录，构建本地镜像，然后让生产 compose 使用该镜像。

### 1. 单独拉取源码

```bash
cd /root
git clone -b glm-token-billing-multiplier https://github.com/LightGao-Hub/new-api.git new-api-src
```

如果目录已存在，更新即可：

```bash
cd /root/new-api-src
git fetch origin
git checkout glm-token-billing-multiplier
git pull
```

### 2. 构建生产本地镜像

```bash
cd /root/new-api-src
docker build -t new-api:glm-token-billing .
```

构建完成后可检查：

```bash
docker images
```

应看到：

```text
new-api:glm-token-billing
```

### 3. 备份生产 compose

```bash
cd /root/new-api
cp docker-compose.yml docker-compose.yml.bak-$(date +%F-%H%M%S)
```

### 4. 修改生产 compose

编辑 `/root/new-api/docker-compose.yml`，只改 `new-api` 服务的镜像。

将：

```yaml
image: calciumion/new-api:latest
```

改为：

```yaml
image: new-api:glm-token-billing
```

保留原有数据库、Redis、端口、挂载目录和环境变量配置。例如这些目录仍然继续使用：

```yaml
volumes:
  - ./data:/data
  - ./logs:/app/logs
```

### 5. 准备倍率配置

在生产部署目录创建配置文件：

```bash
cd /root/new-api
cp /root/new-api-src/config/token_billing_tiers.example.json data/token_billing_tiers.json
```

如果 `data/token_billing_tiers.json` 不存在，程序会使用代码内置默认倍率。

### 6. 只重启应用服务

```bash
cd /root/new-api
docker compose up -d --force-recreate new-api
```

该命令只重建并重启 `new-api` 应用容器，不会重建 PostgreSQL 和 Redis 服务。生产数据目录仍然是：

```text
/root/new-api/data
/root/new-api/logs
/root/new-api/postgres
/root/new-api/redis
```

### 7. 回滚方式

保留旧的官方镜像，不要急着删除。需要回滚时，把 compose 改回：

```yaml
image: calciumion/new-api:latest
```

然后执行：

```bash
cd /root/new-api
docker compose up -d --force-recreate new-api
```

## 验证建议

测试 `glm-5.1` 或 `glm-5.2` 请求时，重点检查：

- `/usage-logs/common` 页面显示的 prompt/completion token 是否为调整后数量
- 接口返回的 OpenAI `usage` 是否为调整后数量
- `/v1/responses` 协议返回的 `usage.input_tokens` / `usage.output_tokens` / `usage.total_tokens` 是否为调整后数量
- 短输入但有大量缓存命中的请求，不应仅因为 `cached_tokens` 很大而触发高倍率

如果下游中转站依赖上游返回的 `usage` 计费，以上返回值会决定下游记录到的 token 数量。
