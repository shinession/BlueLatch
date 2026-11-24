# English

## BlueLatch

BlueLatch connects a paired Bluetooth audio device on Windows and keeps retrying until the connection holds. It is for the device that shows "Not connected" in Windows Settings after the first time and needs several clicks on **Connect**.

### Status

Version 0.1.1: a command line (`bluelatch.exe`) and a window (`bluelatch-gui.exe`).

Verified on a real PC (Windows 11, Intel radio):

- Listing the paired audio devices, in an elevated process and in a normal one.
- Connecting a speaker that was not connected: the retry loop connected it on the third attempt, after 46.3 s (run by the user).
- The window with made-up devices (demo mode): the progress, the connection after 3 attempts, and closing the window while a request is in progress, which waits for that request. The window also listed the real devices, read-only.

**Not verified yet:** the **Connect** and **Cancel** buttons of the window against a real device; earbuds that also have Hands-Free; and whether the reconnect request needs administrator rights. See the live checks in [docs/runbook.md](docs/runbook.md).

### Requirements

- Windows with a Bluetooth radio (tried on Windows 11 only). The device must be paired in Windows Settings first.
- For the window: the Microsoft Edge WebView2 Runtime, which is part of Windows 11.
- To build: Go 1.27 or newer. No C compiler is needed.

### The window

Start `release\bluelatch-gui.exe`. It lists the paired audio devices and shows whether each is connected. **Connect** retries the connection of that device until it holds: the card shows the attempt, the elapsed time, and what Windows did. **Cancel** stops it. One device is connected at a time. The window follows the dark or light theme and the language of Windows (English or Chinese). Closing the window while Windows is working on a request waits for that request, which can take a few seconds.

To try it without touching a real device: `bluelatch-gui.exe -demo -demo-connect "Demo Earbuds"`. All flags are in [docs/runbook.md](docs/runbook.md).

### The command line

```text
bluelatch list [-all] [-v]
bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
bluelatch version
```

`list` shows the paired audio devices with their state. `-all` also shows the other devices Windows reports, `-v` adds the Windows flags, the class, and the installed services.

`connect` finds the device by address or by name (a whole name, or a part of it that is unique; case is ignored). It asks Windows to connect the device, waits, and asks again until the device has stayed connected for `-stable`. Stop it with Ctrl+C. One attempt waits at most `-attempt-timeout` for the link. The pause between attempts grows from 2 s to 5 s. `-max-attempts` limits the attempts; the default 0 means no limit. A device that is connected already is reported at once and is not touched.

Exit codes: 0 success, 1 failure or stopped, 2 wrong command line.

### How it works

Windows has no documented call that simply connects a classic audio device. BlueLatch switches the audio services of the device (A2DP, Hands-Free, Headset) off and on again through the Win32 Bluetooth API, which asks Windows to connect them (ADR 0001). Then it reads the connection state until it holds. A request takes about 6.5 s, and a service is switched off for most of that time; if BlueLatch is stopped in the middle, the next start switches the service on again (ADR 0005). Every attempt is logged with the Windows error codes to `%LocalAppData%\BlueLatch\bluelatch.log`.

### Limits

- Classic Bluetooth audio devices only: headphones, earbuds, speakers (ADR 0002). Bluetooth LE devices, such as most mice and keyboards, are not listed.
- A device that is switched off, in its case, or out of range cannot be connected. BlueLatch keeps trying and connects as soon as the device is available.
- The window has no tray icon, no start with Windows, and no Disconnect button.

### Build and test

```powershell
.\scripts\build.ps1 -Version 0.1.1   # writes release\bluelatch.exe and release\bluelatch-gui.exe
go test ./... -count=1
```

### Documentation

[docs/README.md](docs/README.md) is the index. The engineering rules for contributors and AI agents are in [AGENTS.md](AGENTS.md).

---

# 中文

## BlueLatch

BlueLatch 在 Windows 上连接已配对的蓝牙音频设备，并不断重试，直到连接稳定。它面向这样的设备：第一次之后在 Windows 设置里显示"未连接"，需要多次点击 **Connect**。

### 状态

0.1.1 版：一个命令行（`bluelatch.exe`）和一个窗口（`bluelatch-gui.exe`）。

已在一台真实的电脑上验证（Windows 11，Intel 蓝牙适配器）：

- 在已提权的进程和普通进程中列出已配对的音频设备。
- 连接一台未连接的音箱：重试循环在第三次尝试时连上了，用时 46.3 秒（由用户运行）。
- 窗口使用虚构设备（演示模式）：进度显示、经过 3 次尝试连上，以及在请求进行中关闭窗口（它会等待该请求）。窗口也以只读方式列出了真实设备。

**尚未验证：**窗口的 **Connect** 和 **Cancel** 按钮对真实设备的效果；同时带有 Hands-Free 的耳塞；以及重新连接请求是否需要管理员权限。参见 [docs/runbook.md](docs/runbook.md) 中的真机检查。

### 要求

- 带蓝牙适配器的 Windows（只在 Windows 11 上试过）。设备必须先在 Windows 设置中完成配对。
- 使用窗口需要 Microsoft Edge WebView2 运行时，它是 Windows 11 的一部分。
- 构建需要 Go 1.27 或更高版本，不需要 C 编译器。

### 窗口

启动 `release\bluelatch-gui.exe`。它列出已配对的音频设备，并显示每个设备是否已连接。**Connect** 会重试该设备的连接，直到连接稳定：卡片上显示尝试次数、已用时间以及 Windows 所做的操作。**Cancel** 停止重试。同一时间只连接一个设备。窗口跟随 Windows 的深色或浅色主题和语言（英文或中文）。在 Windows 正在处理请求时关闭窗口，会等待该请求结束，这可能要几秒钟。

不触碰真实设备也能试用：`bluelatch-gui.exe -demo -demo-connect "Demo Earbuds"`。所有参数见 [docs/runbook.md](docs/runbook.md)。

### 命令行

```text
bluelatch list [-all] [-v]
bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
bluelatch version
```

`list` 显示已配对的音频设备及其状态。`-all` 同时显示 Windows 报告的其他设备，`-v` 增加 Windows 标志、设备类别和已安装的服务。

`connect` 按地址或名称查找设备（完整名称，或者唯一的名称片段；不区分大小写）。它请求 Windows 连接设备，等待，再次请求，直到设备连续保持连接 `-stable` 这么久。用 Ctrl+C 停止。一次尝试最多等待 `-attempt-timeout` 让连接建立。两次尝试之间的间隔从 2 秒逐步增加到 5 秒。`-max-attempts` 限制尝试次数，默认的 0 表示不限制。已经连接的设备会立即报告，并且不会被触碰。

退出码：0 成功，1 失败或被停止，2 命令行有误。

### 工作原理

Windows 没有文档化的调用可以直接连接经典蓝牙音频设备。BlueLatch 通过 Win32 蓝牙 API 把设备的音频服务（A2DP、Hands-Free、Headset）先关闭再打开，这会请求 Windows 去连接它们（ADR 0001）。然后读取连接状态，直到连接稳定。一个请求约 6.5 秒，其中大部分时间服务处于关闭状态；如果 BlueLatch 在中途被停止，下一次启动会把服务重新打开（ADR 0005）。每次尝试都会连同 Windows 错误码记录到 `%LocalAppData%\BlueLatch\bluelatch.log`。

### 限制

- 只支持经典蓝牙音频设备：耳机、耳塞、音箱（ADR 0002）。蓝牙 LE 设备（例如大多数鼠标和键盘）不会列出。
- 已关机、在充电盒里或不在范围内的设备无法连接。BlueLatch 会不断重试，设备一可用就会连上。
- 窗口没有托盘图标，没有随 Windows 启动，也没有 Disconnect 按钮。

### 构建与测试

```powershell
.\scripts\build.ps1 -Version 0.1.1   # writes release\bluelatch.exe and release\bluelatch-gui.exe
go test ./... -count=1
```

### 文档

[docs/README.md](docs/README.md) 是索引。面向贡献者和 AI Agent 的工程规则在 [AGENTS.md](AGENTS.md) 中。
