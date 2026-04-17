# 集群示例

本目录提供一个三节点集群启动示例，用于本地验证芙卡洛斯的集群运行行为。

## 启动内容

- 节点 1：HTTP `:8500`，前端 `:2019`
- 节点 2：HTTP `:8501`，前端 `:2020`
- 节点 3：HTTP `:8502`，前端 `:2021`

示例使用 [`configs`](./configs) 下的配置文件，当前目标模式为 `cluster + cp`。

## 文件说明

| 文件 | 用途 |
| --- | --- |
| [`start.bat`](./start.bat) | 构建服务端、清理旧数据、启动三个后端节点、注册成员，再启动三个前端开发服务 |
| [`restart.bat`](./restart.bat) | 停掉已有示例进程并重新启动 |
| [`start.sh`](./start.sh) | Linux 或 macOS 下启动三个后端节点并注册成员 |
| [`configs/node1.yaml`](./configs/node1.yaml) | 引导节点配置 |
| [`configs/node2.yaml`](./configs/node2.yaml) | 跟随节点配置 |
| [`configs/node3.yaml`](./configs/node3.yaml) | 跟随节点配置 |

## Windows 启动

```bat
examples\cluster\start.bat
```

脚本执行顺序：

1. 关闭旧的示例窗口
2. 删除 `data/node1`、`data/node2`、`data/node3`
3. 构建 `registry-server.exe`
4. 启动三个后端节点
5. 登录节点 1，并把节点 2、节点 3 注册为集群成员
6. 启动三个前端开发服务

## Linux 或 macOS 启动

```bash
chmod +x ./examples/cluster/start.sh
./examples/cluster/start.sh
```

Shell 脚本只启动后端节点，日志写入：

- `/tmp/registry-node1.log`
- `/tmp/registry-node2.log`
- `/tmp/registry-node3.log`

## 使用说明

- `start.bat` 启动前会删除本地集群数据，适合做干净启动，不适合保留状态。
- `restart.bat` 默认不清理数据目录，适合代码修改后的重启验证。
- 前端窗口依赖本地 Node.js 和 `web` 目录下的依赖已经安装完成。
- 集群成员注册步骤默认节点 1 可通过 `http://127.0.0.1:8500` 访问。

## 相关文档

- [部署指南](../../docs/deployment_zh-CN.md)
- [English](./README.md)
