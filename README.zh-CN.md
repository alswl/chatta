# chatta

[![CI](https://github.com/alswl/chatta/actions/workflows/ci.yml/badge.svg)](https://github.com/alswl/chatta/actions/workflows/ci.yml)
[![最新版本](https://img.shields.io/github/v/release/alswl/chatta)](https://github.com/alswl/chatta/releases)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)

[English](README.md)

Chatta 是一个 Go CLI 和本地消息总线，用于协调多个编码 Agent 会话。它提供
一组简洁、可脚本化的命令，用于宣布工作、加入项目频道、发送私信、读取收件箱，
以及恢复属于当前会话的客户端。

传输层保持本地化：Chatta 自己在进程内实现 IRC 客户端协议，并使用
[`ngircd`](https://ngircd.barton.de/) 作为共享消息总线。IRC 客户端是内建在
Chatta CLI 里的实现细节，无需单独安装客户端程序。它适合由同一用户
控制的 Agent，在同一台机器或受信任的私有局域网中协作；不是公共聊天服务，也不
是带认证的 Agent-to-Agent 协议替代品。

<img width="960" src="docs/chat-architecture.svg" alt="Chatta 本地 Agent 协作工作原理图">

*用户视角：打开会话、发送消息，然后查看收件箱。*

## 核心概念和部署

- `chatta CLI` 是用户操作入口，负责会话、频道、私信和收件箱。
- IRC 是消息模型；`ngircd` 提供本地 IRC 服务端，Chatta 自己在进程内实现连接它
  所需的客户端协议。
- 在 macOS 上，[`chatta-admin`](skills/chatta-admin/SKILL.md) 会把服务端安装为用户级
  `launchd` 服务，使本地 IRC 总线在终端关闭和重新登录后继续运行。

默认部署只监听本机 `127.0.0.1:6667`。同一台机器上的多个 Agent 会话共享这条总线，
用户实际接触到的是频道、私信和收件箱。

## 功能

- 绑定 Agent 所有者的客户端会话、健康检查和自动恢复。
- 共享频道、私信、收件箱读取，以及会实时播报链路中断的持续监听。
- 按工作树隔离客户端目录，避免独立编码会话互相冲突。
- 进程所有权、锁、频道成员管理和保守的客户端清理。
- 提供聊天、收件箱刷新和 macOS 服务端管理技能。

## 安装

### 使用 `npx skills` 安装

安装教 Agent 使用 Chatta 的技能。`chatta-admin` 同时负责安装、升级 CLI，
并在 `chat` 发现服务端异常时恢复共享服务：

```sh
npx skills add alswl/chatta --skill chat --skill chat-refresh \
  --skill chatta-admin --global
```

技能安装完成后，可以让 Agent 安装 Chatta。`npx skills` 安装的是 Agent Skill
文件本身；CLI 二进制由 `chatta-admin` 技能按校验过的 Release 安装。

### 安装 Release 二进制

安装脚本会下载 macOS 或 Linux 下 `amd64` / `arm64` 平台的校验过的二进制：

```sh
curl -fsSL https://raw.githubusercontent.com/alswl/chatta/master/install.sh | sh
```

指定版本或安装目录：

```sh
CHATTA_VERSION=v0.1.0 CHATTA_INSTALL_DIR="$HOME/.local/bin" \
  sh -c 'curl -fsSL https://raw.githubusercontent.com/alswl/chatta/master/install.sh | sh'
```

### 从源码构建

```sh
git clone https://github.com/alswl/chatta.git
cd chatta
go install ./cmd/chatta
```

仓库也提供 `make build` 和 `make install` 目标。

## 前置依赖

`chat` 命令要求每台参与协作的主机安装 IRC 服务端。客户端部分已内建在 Chatta
里，无需单独安装传输程序：

```sh
# macOS + Homebrew
brew install ngircd
```

Linux 请使用发行版对应的包管理器安装等价软件。Chatta 只检查这些依赖，
不会自动安装系统软件。

> **从旧版本 Chatta 升级**：升级前启动的会话依赖外部 `ii` 客户端，与新的
> 进程内传输不兼容。升级后请重启一次现有会话——
> `chatta chat session stop --force && chatta chat session start <nick>`——
> 之后即可卸载 `ii`。

## 快速开始

使用仓库内置配置启动本地服务器：

```sh
mkdir -p "$HOME/.irc-agent"
cp assets/ngircd-agent-chat.conf "$HOME/.irc-agent/ngircd.conf"
ngircd --configtest --config "$HOME/.irc-agent/ngircd.conf"
ngircd --nodaemon --config "$HOME/.irc-agent/ngircd.conf"
```

让服务器在一个终端中运行，再在另一个终端启动客户端会话：

```sh
chatta chat session start agent-a "project coordination"
chatta chat session status
chatta chat channel join project
chatta chat message send --channel project \
  '[HELLO] agent-a -> all: ready to coordinate.'
```

如果需要由 macOS `launchd` 持久管理服务器，请使用
[chatta-admin 技能](skills/chatta-admin/SKILL.md)。它会校验配置、安装用户级
服务，并让 `ngircd` 在终端关闭和重新登录后继续运行。

## 命令概览

分组命令树是公开接口：

```text
chatta chat session start <nick> [role]
chatta chat session status [--deep]
chatta chat session stop [--force]

chatta chat channel join <channel>
chatta chat channel leave <channel> [reason]
chatta chat channel members [channel]

chatta chat message send <text> [--channel <channel>]
chatta chat message direct <nick> <text>

chatta chat inbox read [--all]
chatta chat inbox watch

chatta chat client list
chatta chat client gc [--dry-run] [--prune]
```

一次典型的协作流程：

```sh
chatta chat channel members project
chatta chat message direct agent-b \
  '[ASK] agent-a -> agent-b: is the API contract ready?'
chatta chat inbox read
chatta chat session stop
```

使用 `chatta chat --help` 或具体子命令的 `--help` 查看参数。隐藏的扁平命令只
作为兼容别名保留；新脚本和文档应使用上面的分组命令。

## 配置

配置优先级从高到低为：

1. CLI 显式参数。
2. `CHATTA_*` 环境变量。
3. 用户配置文件 `$XDG_CONFIG_HOME/chatta/config.yaml`（实际位置遵循平台的
   用户配置目录）。
4. 内置默认值。

聊天配置项如下：

| 配置项 | 环境变量 | CLI 参数 | 默认值 |
| --- | --- | --- | --- |
| 客户端目录 | `CHATTA_CHAT_HOME` | `--home` | 按工作树生成 |
| 服务端地址 | `CHATTA_CHAT_HOST` | `--host` | `127.0.0.1` |
| 服务端端口 | `CHATTA_CHAT_PORT` | `--port` | `6667` |
| 主频道 | `CHATTA_CHAT_CHANNEL` | `--channel` | `#agents` |

旧版 `AGENT_CHAT_*` 环境变量仍作为迁移别名支持。全局 `--config` 可以指定其他
配置文件，`--verbose` 用于开启详细诊断输出。

## 技能和文档

- [`docs/chat.md`](docs/chat.md) — 操作指南和信任边界。
- [`skills/chat/SKILL.md`](skills/chat/SKILL.md) — Agent 侧聊天工作流。
- [`skills/chat-refresh/SKILL.md`](skills/chat-refresh/SKILL.md) — 无后台监视器时的
  单次收件箱检查。
- [`skills/chatta-admin/SKILL.md`](skills/chatta-admin/SKILL.md) — macOS 上持久化
  管理 `ngircd` 服务端。
- [`assets/ngircd-agent-chat.conf`](assets/ngircd-agent-chat.conf) — 默认的仅监听
  loopback 地址的服务端配置。

## 信任边界

内置服务端配置没有密码和 TLS，默认只监听 `127.0.0.1`。如果把 `Listen` 改为
局域网地址，所有能访问 `6667` 端口的主机都可以读取和发送消息。

不要把这套配置暴露到公网或不受信任的网络。它不是 A2A：没有认证身份、能力发现、
结构化任务生命周期，也没有跨组织安全模型。

重启服务端会影响本机上的所有 Agent 会话。IRC 不会重放中断期间发送的消息，
因此重启前应先说明影响。

## 开发

要求 Go 1.22 或更高版本。

```sh
make test
make build
make check-skill
./bin/chatta --help
./bin/chatta version
```

CI 会执行 Go 构建、测试、技能检查和 Go lint。需要真实 `ngircd` 以及
Chatta 二进制的本地 quick-start 场景可运行：

```sh
make check-skill-scenarios
```
