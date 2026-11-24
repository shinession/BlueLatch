# English

## Changelog

### 0.1.2 (2026-11-24)

Added:

- The icon: Snap on a dark tile (ADR 0006). `assets/logo/bluelatch.svg` is the source and `assets/logo/png` has the frames. Both programs carry the icon and their version information; the window shows the icon in its title bar, on its taskbar button, and in the header of its page.
- Disconnect (ADR 0007): a **Disconnect** button on the card of a connected device, and `bluelatch disconnect [-v] [-wait 5s] <name or address>`. Behind them are `Engine.Disconnect`, `Adapter.Disconnect` (`IOCTL_BTH_DISCONNECT_DEVICE`), and `latch.Policy.DisconnectTimeout`.
- `-demo-disconnect NAME` for the demo mode of the window.
- `tools/mkrsrc`, which writes the resource files with the standard library only, `scripts\resources.ps1`, and `scripts\logo-frames.ps1`.

Changed:

- The window runs one action at a time: Disconnect is refused while a connection is being retried, and Connect while a disconnection runs (the buttons are disabled).
- The demo devices keep their state: a device that was disconnected needs its requests again to connect.
- The error of `bluelatch connect` for a paired device that is not an audio device now reads "BlueLatch handles classic Bluetooth audio devices only". The exit code is the same.

Not verified yet: Disconnect on a connected device (the runbook has the check).

Upgrading: copy `bluelatch.exe` and `bluelatch-gui.exe` from `release\`. Nothing has to be migrated. When building from the source, the four `rsrc_windows_*.syso` files are linked by `go build` by itself.

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

### 0.1.2（2026-11-24）

新增：

- 图标：深色底的 Snap（ADR 0006）。`assets/logo/bluelatch.svg` 是源文件，`assets/logo/png` 是各尺寸的帧。两个程序都带有图标和各自的版本信息；窗口在标题栏、任务栏按钮和页面头部显示图标。
- Disconnect（ADR 0007）：已连接设备的卡片上有 **Disconnect** 按钮，命令行有 `bluelatch disconnect [-v] [-wait 5s] <name or address>`。它们背后是 `Engine.Disconnect`、`Adapter.Disconnect`（`IOCTL_BTH_DISCONNECT_DEVICE`）和 `latch.Policy.DisconnectTimeout`。
- 窗口演示模式的 `-demo-disconnect NAME`。
- `tools/mkrsrc`（只用标准库写出资源文件）、`scripts\resources.ps1` 和 `scripts\logo-frames.ps1`。

变更：

- 窗口同一时间只执行一个操作：重试连接期间拒绝 Disconnect，断开期间拒绝 Connect（按钮被禁用）。
- 演示设备会保持状态：被断开的设备需要重新发出请求才能连上。
- 对不是音频设备的已配对设备，`bluelatch connect` 的错误消息现在写作 "BlueLatch handles classic Bluetooth audio devices only"。退出码不变。

尚未验证：在已连接的设备上执行 Disconnect（运行手册中有该检查）。

升级：从 `release\` 复制 `bluelatch.exe` 和 `bluelatch-gui.exe`。不需要迁移任何东西。从源码构建时，四个 `rsrc_windows_*.syso` 文件由 `go build` 自动链接。

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
