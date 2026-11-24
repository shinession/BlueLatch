# English

## Changelog

### 0.1.1 (2026-11-24)

Added:

- The window, `bluelatch-gui.exe`: the paired audio devices, a **Connect** button for each that retries until the connection holds, the progress, and **Cancel** (ADR 0003, ADR 0004).
- The journal of service switches and the repair at start-up, in `bluelatch connect` and in the window (ADR 0005).
- A demo mode of the window with made-up devices (`-demo`, `-demo-connect`).
- `internal/app` (the state of the window and the connection job), `internal/ui`, `internal/appdir`.
- The result of the first real run of `connect` in ADR 0001 and in the runbook.

Changed:

- `scripts\build.ps1` builds both programs.
- `bluelatch connect` switches on the services an earlier run left switched off before it connects (and says so).

Upgrading: copy `bluelatch.exe` and `bluelatch-gui.exe` from `release\`. Nothing has to be migrated.

### 0.1.0 (2026-11-24)

The spike: `bluelatch list`, `bluelatch connect`, the retry engine, the log file, and the documentation (ADR 0001, ADR 0002).

---

# 中文

## 变更日志

### 0.1.1（2026-11-24）

新增：

- 窗口 `bluelatch-gui.exe`：已配对的音频设备，每个设备一个 **Connect** 按钮，重试直到连接稳定，显示进度，并带 **Cancel**（ADR 0003、ADR 0004）。
- 服务开关的待恢复记录，以及在 `bluelatch connect` 和窗口启动时的修复（ADR 0005）。
- 窗口的演示模式，使用虚构设备（`-demo`、`-demo-connect`）。
- `internal/app`（窗口的状态和连接任务）、`internal/ui`、`internal/appdir`。
- 首次真实运行 `connect` 的结果，写入了 ADR 0001 和运行手册。

变更：

- `scripts\build.ps1` 会构建两个程序。
- `bluelatch connect` 在连接之前，会把之前某次运行遗留的已关闭服务重新打开（并说明这一点）。

升级：从 `release\` 复制 `bluelatch.exe` 和 `bluelatch-gui.exe`。不需要迁移任何东西。

### 0.1.0（2026-11-24）

验证原型（spike）：`bluelatch list`、`bluelatch connect`、重试引擎、日志文件，以及文档（ADR 0001、ADR 0002）。
