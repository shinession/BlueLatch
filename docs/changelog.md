# English

## Changelog

### 0.1.4 (2026-11-28)

Added:

- The window lists the sound outputs under **Sound output**, with the current one checked and a **Switch** button on each of the others (ADR 0009). The paired Bluetooth devices follow under **Bluetooth devices**.
- `bluelatch switch [-v] [<name or ID>]`: switches to the named output, or without a name to the next one in the list.
- The console and multimedia defaults move together; the default for calls follows only when it was on the current output.
- The package `internal/audio` (the switching rule and the Core Audio calls), and ADR 0009.
- Made-up sound outputs in the demo mode of the window; they follow the made-up devices.

Verified: `bluelatch switch` moved the default output to a monitor and back on the development PC, from a process that was not elevated. Not verified yet: the **Switch** button of the window against the real outputs, and the outputs of a headset with Hands-Free.

Upgrading: copy `bluelatch.exe` from `release\`. Nothing has to be migrated. `scripts\resources.ps1` was run for 0.1.4.

### 0.1.3 (2026-11-24)

Added:

- Language links at the top of the README (`English · 中文`, and one back to English under the Chinese heading), so that the other language is one click away.
- `bluelatch window [flags]`, which opens the window; `bluelatch` without a command does the same. The flags are those of the former `bluelatch-gui` (`-lang`, `-v`, `-debug`, `-demo`, `-demo-connect`, `-demo-disconnect`).
- ADR 0008, which measured what ADR 0004 had assumed about console and GUI programs.
- Tests for the three states of the console (own, shared with a terminal, none) and for the flags of the window, with fakes; and one that starts a real detached process.

Changed:

- One program: the window is part of `bluelatch.exe` (4.1 MB), and `bluelatch-gui.exe` is gone (ADR 0008, which supersedes ADR 0004). The program stays a console program, so `cmd.exe` and PowerShell wait for the command line and see its exit codes. After a double click a console window shows for about 30 ms before the program closes it. Started from a terminal, the window opens in another process and the prompt comes back at once.
- `bluelatch` without arguments no longer prints the usage and exits with 2: it opens the window. The usage is `bluelatch help`.
- `scripts\build.ps1` builds one program, `scripts\resources.ps1` writes two `.syso` files (amd64 and arm64) instead of four, and `cmd/bluelatch-gui` is gone.
- The documents say what the log of the user's run on a speaker showed: Disconnect and Connect in the window worked (ADR 0007, check 4 of the runbook).

Upgrading: copy `bluelatch.exe` from `release\`, and delete `bluelatch-gui.exe`. Shortcuts to `bluelatch-gui.exe` have to point to `bluelatch.exe`, with no arguments. A script that called `bluelatch` without arguments to get the usage has to call `bluelatch help`. Nothing has to be migrated.

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

### 0.1.4（2026-11-28）

新增：

- 窗口在 **Sound output**（声音输出）下列出声音输出，当前输出有对勾，其他每个有一个 **Switch** 按钮（ADR 0009）。已配对的蓝牙设备在下方的 **Bluetooth devices**（蓝牙设备）之下。
- `bluelatch switch [-v] [<name or ID>]`：切换到指定的输出；不带名称时切到列表中的下一个。
- console 和 multimedia 默认项一起移动；通话默认设备只有本来就在当前输出上时才跟着切换。
- 包 `internal/audio`（切换规则和 Core Audio 调用），以及 ADR 0009。
- 窗口演示模式下的虚构声音输出；它们随虚构设备变化。

已验证：在开发用电脑上，从未提权的进程中，`bluelatch switch` 把默认输出换到显示器又换回。尚未验证：窗口的 **Switch** 按钮对真实输出的效果，以及带 Hands-Free 的耳机的输出。

升级：从 `release\` 复制 `bluelatch.exe`。无需迁移任何东西。已为 0.1.4 运行过 `scripts\resources.ps1`。

新增：

- README 顶部的语言链接（`English · 中文`，以及中文标题下回到英文的链接），一次点击就能切换到另一种语言。
- `bluelatch window [flags]` 打开窗口；不带命令的 `bluelatch` 与它相同。参数就是原来 `bluelatch-gui` 的参数（`-lang`、`-v`、`-debug`、`-demo`、`-demo-connect`、`-demo-disconnect`）。
- ADR 0008，它测量了 ADR 0004 对控制台程序和 GUI 程序所做的假定。
- 用假对象测试控制台的三种状态（独占、与终端共享、没有）和窗口的参数；还有一个测试会真正启动一个独立进程。

变更：

- 一个程序：窗口成为 `bluelatch.exe`（4.1 MB）的一部分，`bluelatch-gui.exe` 不再有（ADR 0008，它取代 ADR 0004）。程序仍是控制台程序，所以 `cmd.exe` 和 PowerShell 会等待命令行并拿到它的退出码。双击之后会有一个控制台窗口显示约 30 毫秒，随后被程序关闭。从终端启动时，窗口在另一个进程中打开，提示符立即回来。
- 不带参数的 `bluelatch` 不再打印用法并返回 2：它打开窗口。用法改用 `bluelatch help`。
- `scripts\build.ps1` 构建一个程序，`scripts\resources.ps1` 写出两个 `.syso` 文件（amd64 和 arm64），而不是四个，`cmd/bluelatch-gui` 已删除。
- 文档记录了用户在一台音箱上运行的日志所显示的内容：窗口里的 Disconnect 和 Connect 都可以工作（ADR 0007，运行手册的检查 4）。

升级：从 `release\` 复制 `bluelatch.exe`，并删除 `bluelatch-gui.exe`。指向 `bluelatch-gui.exe` 的快捷方式要改为指向 `bluelatch.exe`，不带参数。以前用不带参数的 `bluelatch` 来获取用法的脚本，要改为调用 `bluelatch help`。不需要迁移任何东西。

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
