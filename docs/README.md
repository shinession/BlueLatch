# English

## BlueLatch documentation index

Look a topic up here before changing or debugging it. Every document is bilingual: the complete English text comes first, the complete Simplified Chinese text second.

| Document | Describes |
|---|---|
| [architecture.md](./architecture.md) | The implemented architecture: packages, how the devices are read, the reconnect request, the disconnect request, the journal, the retry engine, the sound outputs and how the default one is changed, the application layer, how the program starts, the window, the icon and the version information, the log. What is planned is marked as planned; what is not verified yet is listed at the end. |
| [runbook.md](./runbook.md) | Development setup, build, changing the icon or the version, running the window and its demo mode, checking the page without Windows, the live checks on a real device and on the real sound outputs (and what they showed), the log file, troubleshooting. |
| [changelog.md](./changelog.md) | What changed in each version, and what to do when upgrading. |
| [adr/](./adr/README.md) | Architecture decision records: 0001 how a device is reconnected, 0002 what version 1 covers, 0003 the window, 0004 the window is a separate executable, 0005 the journal of service switches, 0006 the icon, 0007 Disconnect, 0008 one executable, 0009 switching the default sound output. |
| [../assets/logo/](../assets/logo/bluelatch.svg) | The icon: `bluelatch.svg` is the source, `png/` has the frames (ADR 0006). The runbook says how to write the frames and the resource files again. |

---

# 中文

## BlueLatch 文档索引

修改或排查某个主题之前，请先在这里查找。所有文档都是双语的：先给出完整的英文，再给出完整的简体中文。

| 文档 | 内容 |
|---|---|
| [architecture.md](./architecture.md) | 已实现的架构：包、如何读取设备、重新连接请求、断开请求、待恢复记录、重试引擎、声音输出以及如何改变默认输出、应用层、程序如何启动、窗口、图标和版本信息、日志。计划中的内容会标明是计划；尚未验证的内容列在最后。 |
| [runbook.md](./runbook.md) | 开发环境、构建、更换图标或版本、运行窗口及其演示模式、不借助 Windows 检查页面、在真实设备和真实声音输出上的真机检查（以及检查的结果）、日志文件、故障排查。 |
| [changelog.md](./changelog.md) | 每个版本的变更，以及升级时要做的事。 |
| [adr/](./adr/README.md) | 架构决策记录：0001 如何重新连接设备，0002 第 1 版覆盖什么，0003 窗口，0004 窗口是独立的可执行文件，0005 服务开关的待恢复记录，0006 图标，0007 Disconnect，0008 一个可执行文件，0009 切换默认声音输出。 |
| [../assets/logo/](../assets/logo/bluelatch.svg) | 图标：`bluelatch.svg` 是源文件，`png/` 是各尺寸的帧（ADR 0006）。运行手册说明了如何重新写出这些帧和资源文件。 |
