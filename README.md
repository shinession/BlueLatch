<p align="center"><img src="assets/logo/png/bluelatch-512.png" alt="BlueLatch" width="128" height="128"></p>

<p align="center"><b>English</b> · <a href="#中文">中文</a></p>

# English

## BlueLatch

BlueLatch connects a paired Bluetooth audio device on Windows and keeps retrying until the connection holds. It is for the device that shows "Not connected" in Windows Settings after the first time and needs several clicks on **Connect**. It can also disconnect the device again.

### Status

Version 0.1.3: one program, `bluelatch.exe`. It is the window (a double click, or `bluelatch` in a terminal) and the command line (`bluelatch list`, `connect`, `disconnect`). Both connect and disconnect a device. The program carries the BlueLatch icon.

Verified on a real PC (Windows 11, Intel radio):

- Listing the paired audio devices, in an elevated process and in a normal one.
- Connecting a speaker that was not connected: the retry loop connected it on the third attempt, after 46.3 s (run by the user).
- The window with made-up devices (demo mode): the progress, the connection after 3 attempts, and closing the window while a request is in progress, which waits for that request. The window also listed the real devices, read-only.
- **Disconnect** and **Connect** in the window on a real speaker (clicked by the user; the log is the record): the disconnect request was accepted at once, Windows reported the speaker as disconnected 0.5 s later, it had not connected again by itself when the Connect came about 5 s later, and that Connect connected it on the first attempt after 8.4 s.
- The disconnection in the window with made-up devices (demo mode): the button, the progress, and the result.
- The request to disconnect, sent as a normal user for earbuds that were not connected: Windows answered "not connected", not an access error.
- The icon: the program carries it with its version information, Windows extracts it at 16 to 256 px, and the title bar of the window shows it.
- One program for both (ADR 0008): a double click, a shortcut, and `bluelatch window` in a terminal open the window. After a double click a terminal window shows for about 30 ms and none stays. From a terminal the prompt comes back at once. The command line still makes `cmd.exe` and PowerShell wait and gives them its exit codes.

**Not verified yet:** the **Cancel** button of the window against a real device; earbuds that also have Hands-Free; `bluelatch disconnect` on a connected device, and whether some device connects again by itself after it was dropped; whether the reconnect request needs administrator rights (the log does not say whether the runs by the user were elevated); and the console window after a double click on a PC without Windows Terminal. See the live checks in [docs/runbook.md](docs/runbook.md).

### Requirements

- Windows with a Bluetooth radio (tried on Windows 11 only). The device must be paired in Windows Settings first.
- For the window: the Microsoft Edge WebView2 Runtime, which is part of Windows 11.
- To build: Go 1.27 or newer. No C compiler is needed.

### The window

Start `release\bluelatch.exe` with a double click, or type `bluelatch` (or `bluelatch window`) in a terminal, where the prompt comes back at once. A console window shows for about 30 ms after a double click, because the program is a console program on purpose (ADR 0008). The window lists the paired audio devices and shows whether each is connected. **Connect** retries the connection of that device until it holds: the card shows the attempt, the elapsed time, and what Windows did. **Cancel** stops it. A device that is connected has a **Disconnect** button instead: it drops the connection, and the device stays paired (ADR 0007). One action runs at a time. The window follows the dark or light theme and the language of Windows (English or Chinese). Closing the window while Windows is working on a request waits for that request, which can take a few seconds.

To try it without touching a real device: `bluelatch window -demo -demo-connect "Demo Earbuds"`, or `bluelatch window -demo -demo-disconnect "Demo Speaker"`. All flags are in [docs/runbook.md](docs/runbook.md).

### The command line

```text
bluelatch
bluelatch window [-lang auto|en|zh] [-v] [-debug] [-demo]
bluelatch list [-all] [-v]
bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
bluelatch disconnect [-v] [-wait 5s] <name or address>
bluelatch version
```

`bluelatch` without a command opens the window, as `bluelatch window` does; `bluelatch help` prints the usage.

`list` shows the paired audio devices with their state. `-all` also shows the other devices Windows reports, `-v` adds the Windows flags, the class, and the installed services.

`connect` finds the device by address or by name (a whole name, or a part of it that is unique; case is ignored). It asks Windows to connect the device, waits, and asks again until the device has stayed connected for `-stable`. Stop it with Ctrl+C. One attempt waits at most `-attempt-timeout` for the link. The pause between attempts grows from 2 s to 5 s. `-max-attempts` limits the attempts; the default 0 means no limit. A device that is connected already is reported at once and is not touched.

`disconnect` finds the device in the same way and asks Windows to drop its connection. It waits up to `-wait` for Windows to report the device as disconnected. The pairing and the services stay as they are, so `bluelatch connect` connects the device again. A device that is not connected is reported as such (exit code 0) and nothing is sent. When the device still reports being connected after `-wait`, the command fails with exit code 1, because some devices connect again by themselves.

Exit codes: 0 success, 1 failure or stopped, 2 wrong command line.

### How it works

Windows has no documented call that simply connects a classic audio device. BlueLatch switches the audio services of the device (A2DP, Hands-Free, Headset) off and on again through the Win32 Bluetooth API, which asks Windows to connect them (ADR 0001). Then it reads the connection state until it holds. A request takes about 6.5 s, and a service is switched off for most of that time; if BlueLatch is stopped in the middle, the next start switches the service on again (ADR 0005). Every attempt is logged with the Windows error codes to `%LocalAppData%\BlueLatch\bluelatch.log`.

Disconnect is a different request: it asks the radio to drop the connection of the device (`IOCTL_BTH_DISCONNECT_DEVICE`) and changes no service and no pairing (ADR 0007).

### Limits

- Classic Bluetooth audio devices only: headphones, earbuds, speakers (ADR 0002). Bluetooth LE devices, such as most mice and keyboards, are not listed.
- A device that is switched off, in its case, or out of range cannot be connected. BlueLatch keeps trying and connects as soon as the device is available.
- A device that is disconnected may connect again by itself. That is up to the device; it has not been tried on a real device yet.
- The window has no tray icon and no start with Windows.

### Build and test

```powershell
.\scripts\build.ps1 -Version 0.1.3   # writes release\bluelatch.exe
go test ./... -count=1
```

The icon and the version information are in committed resource files (`cmd\bluelatch\rsrc_windows_*.syso`), so building needs no other tool. To write them again, after a change of the icon or the version, run `.\scripts\resources.ps1 -Version 0.1.3` (see [docs/runbook.md](docs/runbook.md)).

### Documentation

[docs/README.md](docs/README.md) is the index. The engineering rules for contributors and AI agents are in [AGENTS.md](AGENTS.md).

---

# 中文

<p align="center"><a href="#english">English</a> · <b>中文</b></p>

## BlueLatch

BlueLatch 在 Windows 上连接已配对的蓝牙音频设备，并不断重试，直到连接稳定。它面向这样的设备：第一次之后在 Windows 设置里显示"未连接"，需要多次点击 **Connect**。它也可以再把设备断开。

### 状态

0.1.3 版：一个程序 `bluelatch.exe`。它既是窗口（双击，或在终端里输入 `bluelatch`），也是命令行（`bluelatch list`、`connect`、`disconnect`）。两者都能连接和断开设备。程序带有 BlueLatch 图标。

已在一台真实的电脑上验证（Windows 11，Intel 蓝牙适配器）：

- 在已提权的进程和普通进程中列出已配对的音频设备。
- 连接一台未连接的音箱：重试循环在第三次尝试时连上了，用时 46.3 秒（由用户运行）。
- 窗口使用虚构设备（演示模式）：进度显示、经过 3 次尝试连上，以及在请求进行中关闭窗口（它会等待该请求）。窗口也以只读方式列出了真实设备。
- 在窗口里对一台真实音箱执行 **Disconnect** 和 **Connect**（由用户点击；日志就是记录）：断开请求被立即接受，Windows 在 0.5 秒后报告音箱已断开，约 5 秒后 Connect 到来时它没有自己再连上，而那次 Connect 第一次尝试就在 8.4 秒后把它连上了。
- 窗口使用虚构设备（演示模式）的断开：按钮、进度显示和结果。
- 以普通用户对一副未连接的耳塞发送断开请求：Windows 返回"未连接"，而不是访问错误。
- 图标：程序带有它和版本信息，Windows 能在 16 到 256 像素下取出它，窗口的标题栏也显示它。
- 一个程序兼顾两者（ADR 0008）：双击、快捷方式，以及在终端里输入 `bluelatch window`，都会打开窗口。双击之后会有一个终端窗口显示约 30 毫秒，没有窗口留下。从终端启动时提示符立即回来。命令行仍然让 `cmd.exe` 和 PowerShell 等待它，并把退出码交给它们。

**尚未验证：**窗口的 **Cancel** 按钮对真实设备的效果；同时带有 Hands-Free 的耳塞；在已连接的设备上执行 `bluelatch disconnect`，以及是否有设备被断开后会自己再连上；重新连接请求是否需要管理员权限（日志没有说明用户的运行是否已提权）；以及在没有 Windows Terminal 的电脑上双击之后的控制台窗口。参见 [docs/runbook.md](docs/runbook.md) 中的真机检查。

### 要求

- 带蓝牙适配器的 Windows（只在 Windows 11 上试过）。设备必须先在 Windows 设置中完成配对。
- 使用窗口需要 Microsoft Edge WebView2 运行时，它是 Windows 11 的一部分。
- 构建需要 Go 1.27 或更高版本，不需要 C 编译器。

### 窗口

双击启动 `release\bluelatch.exe`，或者在终端里输入 `bluelatch`（或 `bluelatch window`），此时提示符会立即回来。双击之后会有一个控制台窗口显示约 30 毫秒，因为程序有意做成控制台程序（ADR 0008）。窗口列出已配对的音频设备，并显示每个设备是否已连接。**Connect** 会重试该设备的连接，直到连接稳定：卡片上显示尝试次数、已用时间以及 Windows 所做的操作。**Cancel** 停止重试。已连接的设备则显示 **Disconnect** 按钮：它断开连接，设备仍保持配对（ADR 0007）。同一时间只执行一个操作。窗口跟随 Windows 的深色或浅色主题和语言（英文或中文）。在 Windows 正在处理请求时关闭窗口，会等待该请求结束，这可能要几秒钟。

不触碰真实设备也能试用：`bluelatch window -demo -demo-connect "Demo Earbuds"`，或者 `bluelatch window -demo -demo-disconnect "Demo Speaker"`。所有参数见 [docs/runbook.md](docs/runbook.md)。

### 命令行

```text
bluelatch
bluelatch window [-lang auto|en|zh] [-v] [-debug] [-demo]
bluelatch list [-all] [-v]
bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
bluelatch disconnect [-v] [-wait 5s] <name or address>
bluelatch version
```

不带命令的 `bluelatch` 打开窗口，与 `bluelatch window` 相同；`bluelatch help` 打印用法。

`list` 显示已配对的音频设备及其状态。`-all` 同时显示 Windows 报告的其他设备，`-v` 增加 Windows 标志、设备类别和已安装的服务。

`connect` 按地址或名称查找设备（完整名称，或者唯一的名称片段；不区分大小写）。它请求 Windows 连接设备，等待，再次请求，直到设备连续保持连接 `-stable` 这么久。用 Ctrl+C 停止。一次尝试最多等待 `-attempt-timeout` 让连接建立。两次尝试之间的间隔从 2 秒逐步增加到 5 秒。`-max-attempts` 限制尝试次数，默认的 0 表示不限制。已经连接的设备会立即报告，并且不会被触碰。

`disconnect` 用同样的方式查找设备，并请求 Windows 断开它的连接。它最多等待 `-wait` 这么久，等 Windows 报告设备已断开。配对和服务保持不变，所以 `bluelatch connect` 可以再把设备连上。未连接的设备会如实报告（退出码 0），什么也不发送。如果过了 `-wait` 设备仍显示为已连接，命令以退出码 1 失败，因为有些设备会自己再连上。

退出码：0 成功，1 失败或被停止，2 命令行有误。

### 工作原理

Windows 没有文档化的调用可以直接连接经典蓝牙音频设备。BlueLatch 通过 Win32 蓝牙 API 把设备的音频服务（A2DP、Hands-Free、Headset）先关闭再打开，这会请求 Windows 去连接它们（ADR 0001）。然后读取连接状态，直到连接稳定。一个请求约 6.5 秒，其中大部分时间服务处于关闭状态；如果 BlueLatch 在中途被停止，下一次启动会把服务重新打开（ADR 0005）。每次尝试都会连同 Windows 错误码记录到 `%LocalAppData%\BlueLatch\bluelatch.log`。

Disconnect 是另一种请求：它请求无线电适配器断开设备的连接（`IOCTL_BTH_DISCONNECT_DEVICE`），不改动任何服务和配对（ADR 0007）。

### 限制

- 只支持经典蓝牙音频设备：耳机、耳塞、音箱（ADR 0002）。蓝牙 LE 设备（例如大多数鼠标和键盘）不会列出。
- 已关机、在充电盒里或不在范围内的设备无法连接。BlueLatch 会不断重试，设备一可用就会连上。
- 被断开的设备可能自己再连上。这由设备决定；尚未在真实设备上试过。
- 窗口没有托盘图标，也没有随 Windows 启动。

### 构建与测试

```powershell
.\scripts\build.ps1 -Version 0.1.3   # writes release\bluelatch.exe
go test ./... -count=1
```

图标和版本信息在已提交的资源文件（`cmd\bluelatch\rsrc_windows_*.syso`）中，所以构建不需要其他工具。图标或版本变化后要重新写出它们，运行 `.\scripts\resources.ps1 -Version 0.1.3`（见 [docs/runbook.md](docs/runbook.md)）。

### 文档

[docs/README.md](docs/README.md) 是索引。面向贡献者和 AI Agent 的工程规则在 [AGENTS.md](AGENTS.md) 中。
